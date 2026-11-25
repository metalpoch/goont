package storage

import (
	"context"
	"fmt"
	"time"
)

// scanLockKey identifica el bloqueo de escaneo ("goont").
const scanLockKey int64 = 0x676f6f6e74

// migrateLockKey identifica el bloqueo de migración ("goont" + 1). Es distinto
// del de escaneo: el servidor y los `ont scan` del cron migran ambos al
// arrancar y deben esperarse, no saltarse.
const migrateLockKey int64 = 0x676f6f6e7401

// TryScanLock intenta tomar un bloqueo de sesión en Postgres para que dos
// ciclos de `ont scan` no se solapen. El cron lanza un escaneo cada 5 minutos;
// si uno tarda más, el siguiente se saltaba hasta ahora y ambos escribían
// muestras duplicadas y abrían su propio pool de conexiones.
//
// Devuelve release = nil y ok = false cuando ya hay otro escaneo en curso.
func (s *Store) TryScanLock(ctx context.Context) (release func(), ok bool, err error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("adquirir conexión para el bloqueo: %w", err)
	}

	var locked bool
	if err := conn.QueryRow(ctx, `SELECT pg_try_advisory_lock($1)`, scanLockKey).Scan(&locked); err != nil {
		conn.Release()
		return nil, false, fmt.Errorf("tomar bloqueo de escaneo: %w", err)
	}
	if !locked {
		conn.Release()
		return nil, false, nil
	}

	return func() {
		// Se libera con un contexto propio: el del escaneo puede estar ya
		// cancelado.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, scanLockKey)
		conn.Release()
	}, true, nil
}
