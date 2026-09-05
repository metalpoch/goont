package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Close() {
	s.pool.Close()
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := newPool(ctx, databaseURL)
	if err != nil {
		if !isInvalidDatabase(err) {
			return nil, err
		}

		if err := createDatabase(ctx, databaseURL); err != nil {
			return nil, err
		}

		pool, err = newPool(ctx, databaseURL)
		if err != nil {
			return nil, err
		}
	}

	return pool, nil
}

func newPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func isInvalidDatabase(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "3D000"
	}
	return false
}

func createDatabase(ctx context.Context, databaseURL string) error {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return fmt.Errorf("database url has no database name")
	}

	admin := *u
	admin.Path = "/postgres"

	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		return fmt.Errorf("connect to admin database: %w", err)
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", pgx.Identifier{dbName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("create database %s: %w", dbName, err)
	}

	return nil
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS timescaledb`,

		`CREATE TABLE IF NOT EXISTS olts (
			ip           text PRIMARY KEY,
			community    text NOT NULL,
			name         text NOT NULL DEFAULT '',
			location     text NOT NULL DEFAULT '',
			snmp_timeout int  NOT NULL DEFAULT 60,
			snmp_retries int  NOT NULL DEFAULT 3,
			created_at   timestamptz NOT NULL DEFAULT now(),
			updated_at   timestamptz NOT NULL DEFAULT now()
		)`,

		`CREATE TABLE IF NOT EXISTS onts (
			olt_ip         text NOT NULL,
			gpon_idx       bigint NOT NULL,
			ont_idx        int  NOT NULL,
			serial_number  text NOT NULL DEFAULT '',
			description    text NOT NULL DEFAULT '',
			line_profile   text NOT NULL DEFAULT '',
			gpon_interface text NOT NULL DEFAULT '',
			last_seen      timestamptz NOT NULL DEFAULT now(),
			PRIMARY KEY (olt_ip, gpon_idx, ont_idx)
		)`,

		`CREATE TABLE IF NOT EXISTS ont_measurements (
			time          timestamptz NOT NULL,
			olt_ip        text NOT NULL,
			gpon_idx      bigint NOT NULL,
			ont_idx       int  NOT NULL,
			serial_number text NOT NULL DEFAULT '',
			description   text NOT NULL DEFAULT '',
			line_profile  text NOT NULL DEFAULT '',
			run_status    int,
			olt_distance  int,
			temperature   int,
			tx_power      int,
			rx_power      int,
			bytes_in      bigint,
			bytes_out     bigint
		)`,

		`SELECT create_hypertable('ont_measurements', 'time', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE)`,

		`CREATE INDEX IF NOT EXISTS idx_ont_measurements_ont ON ont_measurements (olt_ip, gpon_idx, ont_idx, time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_ont_measurements_gpon ON ont_measurements (olt_ip, gpon_idx, time DESC)`,

		`ALTER TABLE ont_measurements SET (
			timescaledb.compress,
			timescaledb.compress_segmentby = 'olt_ip, gpon_idx, ont_idx',
			timescaledb.compress_orderby = 'time DESC'
		)`,
		`SELECT add_compression_policy('ont_measurements', INTERVAL '3 days', if_not_exists => TRUE)`,

		`CREATE TABLE IF NOT EXISTS gpon_measurements (
			time      timestamptz NOT NULL,
			olt_ip    text NOT NULL,
			gpon_idx  bigint NOT NULL,
			bytes_in  bigint,
			bytes_out bigint
		)`,

		`SELECT create_hypertable('gpon_measurements', 'time', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE)`,

		`CREATE INDEX IF NOT EXISTS idx_gpon_measurements_gpon ON gpon_measurements (olt_ip, gpon_idx, time DESC)`,

		`ALTER TABLE gpon_measurements SET (
			timescaledb.compress,
			timescaledb.compress_segmentby = 'olt_ip, gpon_idx',
			timescaledb.compress_orderby = 'time DESC'
		)`,
		`SELECT add_compression_policy('gpon_measurements', INTERVAL '7 days', if_not_exists => TRUE)`,
	}

	for _, stmt := range statements {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	if err := migrateGponIdxToBigint(ctx, pool); err != nil {
		return err
	}

	return nil
}

func migrateGponIdxToBigint(ctx context.Context, pool *pgxpool.Pool) error {
	tables := []struct {
		name       string
		hypertable bool
		compress   string
	}{
		{"onts", false, ""},
		{"ont_measurements", true, "3 days"},
		{"gpon_measurements", true, "7 days"},
	}

	for _, t := range tables {
		var dataType string
		err := pool.QueryRow(ctx, `
			SELECT data_type
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'gpon_idx'
		`, t.name).Scan(&dataType)
		if err != nil {
			return fmt.Errorf("lookup %s.gpon_idx type: %w", t.name, err)
		}

		var hasNegative bool
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE gpon_idx < 0)`, t.name)).Scan(&hasNegative); err != nil {
			return fmt.Errorf("check negative %s.gpon_idx: %w", t.name, err)
		}

		if dataType == "bigint" && !hasNegative {
			continue
		}

		if t.hypertable {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`SELECT remove_compression_policy('%s', if_exists => true)`, t.name)); err != nil {
				return fmt.Errorf("remove compression policy %s: %w", t.name, err)
			}
			if _, err := pool.Exec(ctx, fmt.Sprintf(`SELECT decompress_chunk(c, true) FROM show_chunks('%s') c`, t.name)); err != nil {
				return fmt.Errorf("decompress %s: %w", t.name, err)
			}
		}

		if dataType != "bigint" {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN gpon_idx TYPE bigint`, t.name)); err != nil {
				return fmt.Errorf("alter %s.gpon_idx to bigint: %w", t.name, err)
			}
		}

		if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s SET gpon_idx = gpon_idx + 4294967296 WHERE gpon_idx < 0`, t.name)); err != nil {
			return fmt.Errorf("rewrite negative %s.gpon_idx: %w", t.name, err)
		}

		if t.hypertable {
			if _, err := pool.Exec(ctx, fmt.Sprintf(`SELECT add_compression_policy('%s', INTERVAL '%s', if_not_exists => TRUE)`, t.name, t.compress)); err != nil {
				return fmt.Errorf("restore compression policy %s: %w", t.name, err)
			}
		}
	}

	return nil
}
