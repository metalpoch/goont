package storage

import (
	"context"
	"sync"
	"testing"
)

// TestMigrate_Concurrent cubre el arranque simultaneo del servidor y de un
// `ont scan` del cron: ambos llaman a Migrate. Sin el bloqueo de sesion los dos
// CREATE INDEX IF NOT EXISTS veian el indice como inexistente y el segundo
// fallaba con "duplicate key value violates unique constraint
// pg_class_relname_nsp_index" (SQLSTATE 23505). Se borra un indice antes para
// que las creaciones concurrentes compitan de verdad.
func TestMigrate_Concurrent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	if _, err := s.pool.Exec(ctx, `DROP INDEX IF EXISTS idx_gpon_measurements_time`); err != nil {
		t.Fatalf("drop index: %v", err)
	}

	errs := make([]error, 3)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = Migrate(ctx, s.pool) })
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Migrate concurrente #%d: %v", i, err)
		}
	}

	var indexes int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_gpon_measurements_time'
	`).Scan(&indexes); err != nil {
		t.Fatalf("lookup index: %v", err)
	}
	if indexes != 1 {
		t.Fatalf("el indice deberia existir tras migrar en paralelo, hay %d", indexes)
	}
}

// TestLiftDecompressionLimit comprueba que el backfill puede descomprimir mas
// tuplas de las que TimescaleDB permite por sentencia: sin esto la migracion
// aborta con SQLSTATE 53400 en cuanto gpon_measurements tiene chunks
// comprimidos. El ajuste es SET LOCAL, asi que se revierte al cerrar la
// transaccion.
func TestLiftDecompressionLimit(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const setting = "timescaledb.max_tuples_decompressed_per_dml_transaction"

	var before string
	err := s.pool.QueryRow(ctx, `SELECT current_setting($1)`, setting).Scan(&before)
	if err != nil {
		t.Skipf("TimescaleDB no disponible: %v", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := liftDecompressionLimit(ctx, tx); err != nil {
		t.Fatalf("liftDecompressionLimit: %v", err)
	}

	var during string
	if err := tx.QueryRow(ctx, `SELECT current_setting($1)`, setting).Scan(&during); err != nil {
		t.Fatalf("current_setting dentro de la transaccion: %v", err)
	}
	if during != "0" {
		t.Fatalf("se esperaba el limite desactivado (0) dentro de la transaccion, es %q", during)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	var after string
	if err := s.pool.QueryRow(ctx, `SELECT current_setting($1)`, setting).Scan(&after); err != nil {
		t.Fatalf("current_setting tras el rollback: %v", err)
	}
	if after != before {
		t.Fatalf("el ajuste deberia revertirse: antes %q, despues %q", before, after)
	}
}
