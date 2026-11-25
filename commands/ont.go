package commands

import (
	"context"
	"fmt"
	"goont/config"
	"goont/models"
	"goont/snmp"
	"goont/storage"
	"log"
	"math/rand"
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

		release, ok, err := store.TryScanLock(ctx)
		if err != nil {
			return fmt.Errorf("Error al bloquear el escaneo: %v", err)
		}
		if !ok {
			log.Println("Ya hay un escaneo en curso; se omite este ciclo")
			return nil
		}
		defer release()

		olts, err := store.GetOLTs(ctx)
		if err != nil {
			return fmt.Errorf("Error al intentar obtener todos los OLT: %v", err)
		}

		// Rotar el orden para que los OLTs lentos no queden siempre fuera del
		// deadline: el ciclo corta lo que no llegó a escanear.
		rand.Shuffle(len(olts), func(i, j int) { olts[i], olts[j] = olts[j], olts[i] })

		scanCtx, cancel := context.WithTimeout(ctx, cfg.ScanDeadline)
		defer cancel()

		sem := make(chan struct{}, cfg.MaxOLTs)

		var wg sync.WaitGroup
	loop:
		for _, olt := range olts {
			select {
			case sem <- struct{}{}:
			case <-scanCtx.Done():
				break loop
			}

			wg.Go(func() {
				defer func() { <-sem }()
				scanOLT(scanCtx, cfg, store, olt)
			})
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-scanCtx.Done():
			// No se espera a los que siguen trabados en SNMP: cada puerto ya
			// guardó lo suyo y el lock se libera para el próximo cron.
			log.Println("Tiempo máximo de escaneo alcanzado; se conservan las muestras ya guardadas")
		}

		return nil
	},
}

// scanLimits acota el timeout y los reintentos del escaneo. Un OLT configurado
// con 60 s (o 3 reintentos) no debe frenar el ciclo: se usan los valores
// globales salvo que el OLT tenga valores menores.
func scanLimits(cfg config.Config, olt models.OLT) (time.Duration, int) {
	timeout := cfg.ScanTimeout
	if olt.Timeout > 0 && time.Duration(olt.Timeout)*time.Second < timeout {
		timeout = time.Duration(olt.Timeout) * time.Second
	}

	retries := cfg.ScanRetries
	if olt.Retries > 0 && olt.Retries < retries {
		retries = olt.Retries
	}

	return timeout, retries
}

func scanOLT(ctx context.Context, cfg config.Config, store *storage.Store, olt models.OLT) {
	if ctx.Err() != nil {
		return
	}

	timeout, retries := scanLimits(cfg, olt)
	client := snmp.NewSnmp(
		olt.IP,
		olt.Community,
		retries,
		timeout,
		cfg.SNMPConns,
	)
	defer client.Close()

	gpons, err := client.IfNames()
	if err != nil {
		log.Printf("Error al intentar obtener los puertos gpon de %s: %v", olt.IP, err)
		return
	}

	now := time.Now()

	conns := cfg.SNMPConns
	if conns <= 0 {
		conns = snmp.DefaultConns
	}
	sem := make(chan struct{}, conns)

	var wg sync.WaitGroup
portLoop:
	for _, g := range gpons {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break portLoop
		}

		wg.Go(func() {
			defer func() { <-sem }()
			scanPort(ctx, client, store, olt.IP, g, now)
		})
	}

	wg.Wait()
}

// scanPort guarda lo que haya respondido del puerto, sin abortar el OLT:
// primero los ONTs (parciales, con NULL en lo que faltó) y después el tráfico
// del puerto. Si el tráfico falla, los ONTs quedan guardados y el puerto queda
// como hueco en gpon_measurements.
func scanPort(ctx context.Context, client *snmp.Snmp, store *storage.Store, oltIP string, g snmp.Gpon, now time.Time) {
	if ctx.Err() != nil {
		return
	}

	raw, ontErr := client.OntQuery(ctx, g)
	if ontErr != nil {
		log.Printf("error al ejecutar las consultas a los ont de %s del puerto %d (%s): %v", oltIP, g.Idx, g.IfName, ontErr)
	}

	onts := make([]models.Ont, 0, len(raw))
	for idx, o := range raw {
		onts = append(onts, models.Ont{
			Time:             now,
			GponIdx:          g.Idx,
			GponInterface:    g.IfName,
			OntIdx:           idx,
			Despt:            o.Despt,
			SerialNumber:     o.SerialNumber,
			LineProfName:     o.LineProfName,
			ControlRanging:   o.ControlRanging,
			ControlRunStatus: o.ControlRunStatus,
			Temperature:      o.Temperature,
			Tx:               o.Tx,
			Rx:               o.Rx,
			BytesIn:          o.BytesIn,
			BytesOut:         o.BytesOut,
			BytesInValid:     o.BytesInValid,
			BytesOutValid:    o.BytesOutValid,
		})
	}

	if len(onts) > 0 {
		if err := store.UpsertOnts(ctx, oltIP, onts); err != nil {
			log.Printf("Error al intentar registrar los ont de %s del puerto %d (%s): %v", oltIP, g.Idx, g.IfName, err)
		}
		if err := store.InsertOntMeasurements(ctx, oltIP, onts); err != nil {
			log.Printf("Error al intentar almacenar los ont de %s del puerto %d (%s): %v", oltIP, g.Idx, g.IfName, err)
		}
	}

	if ctx.Err() != nil {
		return
	}

	traffic, err := client.GponTrafficPort(ctx, g)
	if err != nil {
		log.Printf("Error al intentar obtener el trafico del puerto %d (%s) de %s: %v", g.Idx, g.IfName, oltIP, err)
		return
	}

	samples := []models.GponSample{{
		Time:     now,
		GponIdx:  g.Idx,
		BytesIn:  traffic.BytesIn,
		BytesOut: traffic.BytesOut,
	}}
	if err := store.InsertGponMeasurements(ctx, oltIP, samples, onts); err != nil {
		log.Printf("Error al intentar almacenar el trafico del puerto %d (%s) de %s: %v", g.Idx, g.IfName, oltIP, err)
	}
}
