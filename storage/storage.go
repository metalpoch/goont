package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

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
	return connect(ctx, databaseURL, dbMaxConns())
}

// ConnectScan crea un pool pequeño para los escaneos SNMP de la CLI. Cada
// ejecución de `ont scan` es efímera (contenedor nuevo) y solo necesita unas
// pocas conexiones: reservar el máximo del servidor agota el pool de Postgres
// cuando los ciclos de cron se solapan.
func ConnectScan(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return connect(ctx, databaseURL, 3)
}

func connect(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	pool, err := newPool(ctx, databaseURL, maxConns)
	if err != nil {
		if !isInvalidDatabase(err) {
			return nil, err
		}

		if err := createDatabase(ctx, databaseURL); err != nil {
			return nil, err
		}

		pool, err = newPool(ctx, databaseURL, maxConns)
		if err != nil {
			return nil, err
		}
	}

	return pool, nil
}

func dbMaxConns() int32 {
	if v := os.Getenv("GOONT_DB_MAX_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return int32(n)
		}
	}
	return 10
}

func newPool(ctx context.Context, databaseURL string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	if maxConns < 1 {
		maxConns = 1
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = 1
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

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

// migrationConn is what Migrate needs to run its DDL. Both *pgxpool.Pool and
// *pgxpool.Conn satisfy it; Migrate uses one pooled connection so that the
// advisory lock serializing it is held by the same session that runs the
// statements.
type migrationConn interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	// The server and every `ont scan` started by cron migrate on startup, so two
	// Migrate calls can overlap. CREATE INDEX IF NOT EXISTS is not atomic against
	// a concurrent identical CREATE INDEX: both see the index as missing and the
	// loser fails with a pg_class unique violation (SQLSTATE 23505). Serialize
	// with a session advisory lock instead of racing.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrateLockKey); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer func() {
		// Released with a context of its own: the caller's may be cancelled and
		// a leaked advisory lock would block every later start.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrateLockKey)
	}()

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
		// No se crea un indice (olt_ip, time) en ont_measurements: ninguna
		// consulta filtra la hypertable por OLT sin pasar por gpon_idx (los
		// conteos por estado ya salen de gpon_measurements y el detalle por
		// ONT usa idx_ont_measurements_ont). En una tabla grande ese CREATE
		// INDEX bloquea las escrituras durante minutos y no aporta nada.

		`ALTER TABLE ont_measurements SET (
			timescaledb.compress,
			timescaledb.compress_segmentby = 'olt_ip, gpon_idx, ont_idx',
			timescaledb.compress_orderby = 'time DESC'
		)`,
		`SELECT add_compression_policy('ont_measurements', INTERVAL '3 days', if_not_exists => TRUE)`,

		`CREATE TABLE IF NOT EXISTS gpon_measurements (
			time           timestamptz NOT NULL,
			olt_ip         text NOT NULL,
			gpon_idx       bigint NOT NULL,
			bytes_in       bigint,
			bytes_out      bigint,
			count_active   integer NOT NULL DEFAULT 0,
			count_inactive integer NOT NULL DEFAULT 0,
			count_error    integer NOT NULL DEFAULT 0
		)`,

		`SELECT create_hypertable('gpon_measurements', 'time', chunk_time_interval => INTERVAL '1 day', if_not_exists => TRUE)`,

		`CREATE INDEX IF NOT EXISTS idx_gpon_measurements_gpon ON gpon_measurements (olt_ip, gpon_idx, time DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_gpon_measurements_time ON gpon_measurements (olt_ip, time DESC)`,

		`ALTER TABLE gpon_measurements SET (
			timescaledb.compress,
			timescaledb.compress_segmentby = 'olt_ip, gpon_idx',
			timescaledb.compress_orderby = 'time DESC'
		)`,
		`SELECT add_compression_policy('gpon_measurements', INTERVAL '7 days', if_not_exists => TRUE)`,
	}

	for _, stmt := range statements {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	if err := migrateGponIdxToBigint(ctx, conn); err != nil {
		return err
	}

	if err := migrateGponCounts(ctx, conn); err != nil {
		return err
	}

	return nil
}

// rowQuerier is implemented by both *pgxpool.Pool and pgx.Tx: the migration
// checks the schema before opening its transaction and again once it holds the
// table lock.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// gponCountsColumnExists reports whether gpon_measurements already has the
// per-port status counters.
func gponCountsColumnExists(ctx context.Context, q rowQuerier) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'gpon_measurements'
			  AND column_name = 'count_active'
		)
	`).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("lookup gpon_measurements.count_active: %w", err)
	}
	return exists, nil
}

// liftDecompressionLimit disables the per-statement cap TimescaleDB puts on the
// number of tuples a DML statement may decompress. The backfill rewrites every
// row of gpon_measurements and its older chunks are compressed, so the default
// cap (100k tuples) aborts it with SQLSTATE 53400 ("tuple decompression limit
// exceeded by operation"). The setting is renamed between releases
// (max_tuples_decompressed_per_dml in older ones), so look up which name this
// server actually has; on a plain PostgreSQL there is none and nothing to do.
// 0 means unlimited. SET LOCAL scopes it to the migration transaction.
func liftDecompressionLimit(ctx context.Context, tx pgx.Tx) error {
	var name string
	err := tx.QueryRow(ctx, `
		SELECT name FROM pg_settings
		WHERE name IN ('timescaledb.max_tuples_decompressed_per_dml_transaction',
		               'timescaledb.max_tuples_decompressed_per_dml')
		ORDER BY length(name) DESC
		LIMIT 1
	`).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup tuple decompression limit: %w", err)
	}
	// The name comes from pg_settings, never from user input.
	if _, err := tx.Exec(ctx, `SET LOCAL `+name+` = 0`); err != nil {
		return fmt.Errorf("lift tuple decompression limit: %w", err)
	}
	return nil
}

// migrateGponCounts adds the per-port ONT status counters to
// gpon_measurements and fills the history once. The counters used to be
// recomputed from ont_measurements on every traffic query, which scanned one
// row per ONT per scan (~1M rows per OLT per week) and made the 7-day
// endpoints expensive.
//
// The whole migration (ALTER + backfill) runs in one transaction: if it fails
// halfway the columns are rolled back and the next start retries. Once the
// column exists the backfill is skipped, so restarts stay cheap.
func migrateGponCounts(ctx context.Context, conn migrationConn) error {
	// Fast path: a previous start already migrated.
	exists, err := gponCountsColumnExists(ctx, conn)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin gpon counts migration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Take the lock the ALTER would take before deciding anything, so that a
	// concurrent Migrate that added the columns has committed by now and we can
	// skip the backfill. Re-checking *after* the ALTER would be wrong: the check
	// would see the columns this transaction just created.
	if _, err := tx.Exec(ctx, `LOCK TABLE gpon_measurements IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock gpon_measurements: %w", err)
	}

	exists, err = gponCountsColumnExists(ctx, tx)
	if err != nil {
		return err
	}
	if exists {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit gpon counts migration: %w", err)
		}
		return nil
	}

	for _, stmt := range []string{
		`ALTER TABLE gpon_measurements ADD COLUMN IF NOT EXISTS count_active integer NOT NULL DEFAULT 0`,
		`ALTER TABLE gpon_measurements ADD COLUMN IF NOT EXISTS count_inactive integer NOT NULL DEFAULT 0`,
		`ALTER TABLE gpon_measurements ADD COLUMN IF NOT EXISTS count_error integer NOT NULL DEFAULT 0`,
	} {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("add gpon count columns: %w", err)
		}
	}

	if err := liftDecompressionLimit(ctx, tx); err != nil {
		return err
	}

	// One pass over ont_measurements grouped by (olt_ip, gpon_idx, time). ONT
	// rows and GPON rows from the same scan share the timestamp, so this
	// reconstructs the exact counters the new code records at scan time.
	// Measured on a 7-day, 8-OLT bench (8.2M ONT rows -> 258k GPON rows):
	// ~2.6 s.
	if _, err := tx.Exec(ctx, `
		UPDATE gpon_measurements g
		SET count_active   = c.active,
		    count_inactive = c.inactive,
		    count_error    = c.error
		FROM (
			SELECT olt_ip, gpon_idx, time,
			       COUNT(*) FILTER (WHERE run_status = 1)::int  AS active,
			       COUNT(*) FILTER (WHERE run_status = 2)::int  AS inactive,
			       COUNT(*) FILTER (WHERE run_status = -1)::int AS error
			FROM ont_measurements
			GROUP BY olt_ip, gpon_idx, time
		) c
		WHERE g.olt_ip = c.olt_ip
		  AND g.gpon_idx = c.gpon_idx
		  AND g.time = c.time
	`); err != nil {
		return fmt.Errorf("backfill gpon counts: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit gpon counts migration: %w", err)
	}

	return nil
}

func migrateGponIdxToBigint(ctx context.Context, conn migrationConn) error {
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
		err := conn.QueryRow(ctx, `
			SELECT data_type
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = 'gpon_idx'
		`, t.name).Scan(&dataType)
		if err != nil {
			return fmt.Errorf("lookup %s.gpon_idx type: %w", t.name, err)
		}

		var hasNegative bool
		if err := conn.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE gpon_idx < 0)`, t.name)).Scan(&hasNegative); err != nil {
			return fmt.Errorf("check negative %s.gpon_idx: %w", t.name, err)
		}

		if dataType == "bigint" && !hasNegative {
			continue
		}

		if t.hypertable {
			if _, err := conn.Exec(ctx, fmt.Sprintf(`SELECT remove_compression_policy('%s', if_exists => true)`, t.name)); err != nil {
				return fmt.Errorf("remove compression policy %s: %w", t.name, err)
			}
			if _, err := conn.Exec(ctx, fmt.Sprintf(`SELECT decompress_chunk(c, true) FROM show_chunks('%s') c`, t.name)); err != nil {
				return fmt.Errorf("decompress %s: %w", t.name, err)
			}
		}

		if dataType != "bigint" {
			if _, err := conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN gpon_idx TYPE bigint`, t.name)); err != nil {
				return fmt.Errorf("alter %s.gpon_idx to bigint: %w", t.name, err)
			}
		}

		if _, err := conn.Exec(ctx, fmt.Sprintf(`UPDATE %s SET gpon_idx = gpon_idx + 4294967296 WHERE gpon_idx < 0`, t.name)); err != nil {
			return fmt.Errorf("rewrite negative %s.gpon_idx: %w", t.name, err)
		}

		if t.hypertable {
			if _, err := conn.Exec(ctx, fmt.Sprintf(`SELECT add_compression_policy('%s', INTERVAL '%s', if_not_exists => TRUE)`, t.name, t.compress)); err != nil {
				return fmt.Errorf("restore compression policy %s: %w", t.name, err)
			}
		}
	}

	return nil
}
