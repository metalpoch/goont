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

	"github.com/urfave/cli/v3"
)

var OntScan *cli.Command = &cli.Command{
	Name:  "scan",
	Usage: "obtiene los datos de los ONT",
	Action: func(ctx context.Context, c *cli.Command) error {
		cfg := config.Load()

		store, cleanup, err := newStore(ctx)
		if err != nil {
			return err
		}
		defer cleanup()

		olts, err := store.GetOLTs(ctx)
		if err != nil {
			return fmt.Errorf("Error al intentar obtener todos los OLT: %v", err)
		}

		sem := make(chan struct{}, cfg.MaxOLTs)

		var wg sync.WaitGroup
		for _, olt := range olts {
			sem <- struct{}{}

			wg.Go(func() {
				defer func() { <-sem }()

				scanOLT(ctx, cfg, store, olt)
			})
		}

		wg.Wait()
		return nil
	},
}

func scanOLT(ctx context.Context, cfg config.Config, store *storage.Store, olt models.OLT) {
	client := snmp.NewSnmp(
		olt.IP,
		olt.Community,
		olt.Retries,
		time.Duration(olt.Timeout)*time.Second,
		cfg.SNMPConns,
	)
	defer client.Close()

	gpons, err := client.IfNames()
	if err != nil {
		log.Printf("Error al intentar obtener los puertos gpon de %s: %v", olt.IP, err)
		return
	}

	now := time.Now()

	onts := ontScanner(client, gpons, now)
	if len(onts) > 0 {
		if err := store.UpsertOnts(ctx, olt.IP, onts); err != nil {
			log.Printf("Error al intentar registrar los ont de %s (%s): %v", olt.IP, olt.Name, err)
			return
		}

		if err := store.InsertOntMeasurements(ctx, olt.IP, onts); err != nil {
			log.Printf("Error al intentar almacenar los ont de %s (%s) en la base de datos: %v", olt.IP, olt.Name, err)
			return
		}
	}

	traffic, err := client.GponTraffic(gpons)
	if err != nil {
		log.Printf("Error al intentar obtener el trafico de los puertos gpon de %s: %v", olt.IP, err)
		return
	}

	samples := make([]models.GponSample, 0, len(traffic))
	for _, t := range traffic {
		samples = append(samples, models.GponSample{
			Time:     now,
			GponIdx:  t.Idx,
			BytesIn:  t.BytesIn,
			BytesOut: t.BytesOut,
		})
	}

	if err := store.InsertGponMeasurements(ctx, olt.IP, samples); err != nil {
		log.Printf("Error al intentar almacenar el trafico de los puertos gpon de %s (%s) en la base de datos: %v", olt.IP, olt.Name, err)
	}
}
