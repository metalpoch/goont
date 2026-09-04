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
			gpon_idx       int  NOT NULL,
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
			gpon_idx      int  NOT NULL,
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
			gpon_idx  int  NOT NULL,
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

	return nil
}
