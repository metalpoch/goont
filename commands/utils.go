package commands

import (
	"context"
	"fmt"
	"goont/config"
	"goont/models"
	"goont/snmp"
	"goont/storage"
	"log"
	"sync"
	"time"
)

func newStore(ctx context.Context) (*storage.Store, func(), error) {
	cfg := config.Load()

	pool, err := storage.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("Error al intentar conectar a la base de datos: %v", err)
	}

	if err := storage.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("Error al intentar inicializar la base de datos: %v", err)
	}

	return storage.New(pool), pool.Close, nil
}

func ontScanner(olt *snmp.Snmp, gpons []snmp.Gpon, now time.Time) []models.Ont {
	sem := make(chan struct{}, snmp.DefaultConns)
	ontsBuffer := make(chan []models.Ont, len(gpons))

	var wg sync.WaitGroup

	for _, g := range gpons {
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			allOnt, err := olt.OntQuery(g)
			if err != nil {
				log.Printf("error al ejecutar las consultas a los ont de %s del puerto %d (%s): %v\n", olt.IP(), g.Idx, g.IfName, err)
				return
			}

			onts := make([]models.Ont, 0, len(allOnt))
			for idx, ont := range allOnt {
				onts = append(onts, models.Ont{
					Time:             now,
					GponIdx:          g.Idx,
					GponInterface:    g.IfName,
					OntIdx:           idx,
					Despt:            ont.Despt,
					SerialNumber:     ont.SerialNumber,
					LineProfName:     ont.LineProfName,
					ControlRanging:   ont.ControlRanging,
					ControlRunStatus: ont.ControlRunStatus,
					Temperature:      ont.Temperature,
					Tx:               ont.Tx,
					Rx:               ont.Rx,
					BytesIn:          ont.BytesIn,
					BytesOut:         ont.BytesOut,
				})
			}

			ontsBuffer <- onts
		})
	}

	go func() {
		wg.Wait()
		close(ontsBuffer)
	}()

	var result []models.Ont
	for onts := range ontsBuffer {
		result = append(result, onts...)
	}

	return result
}
