package commands

import (
	"context"
	"fmt"
	"goont/config"
	"goont/storage"
)

func newStore(ctx context.Context) (*storage.Store, func(), error) {
	cfg := config.Load()

	pool, err := storage.ConnectScan(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("Error al intentar conectar a la base de datos: %v", err)
	}

	if err := storage.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("Error al intentar inicializar la base de datos: %v", err)
	}

	return storage.New(pool), pool.Close, nil
}
