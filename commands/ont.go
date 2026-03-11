package commands

import (
	"context"
	"fmt"
	"goont/snmp"
	"goont/storage"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/urfave/cli/v3"
)

var OntScan *cli.Command = &cli.Command{
	Name:  "scan",
	Usage: "obtiene los datos de los ONT",
	Action: func(ctx context.Context, c *cli.Command) error {
		dir, err := os.Executable()
		if err != nil {
			return fmt.Errorf("Error al intentar obtener la ruta del ejecutable")
		}

		olts, err := getAllOLTS(dir)
		if err != nil {
			return err
		}

		var wg sync.WaitGroup
		for _, olt := range olts {
			wg.Go(func() {
				dbDir := filepath.Join(filepath.Dir(dir), olt.IP)
				client, err := storage.NewOntDB(dbDir)
				if err != nil {
					log.Printf("Error al intentar acceder a la base de datos: %v", err)
					return
				}

				defer client.Close()

				onts := ontScanner(
					snmp.NewSnmp(
						olt.IP,
						olt.Community,
						olt.Retries,
						time.Duration(olt.Timeout)*time.Second,
					))
				if len(onts) == 0 {
					return
				}

				if err := client.BatchInsertOntMeasurements(onts); err != nil {
					log.Printf("Error al intentar almacenarlos ont de %s (%s) en la base de datos: %v", olt.IP, olt.Name, err)
					return
				}
			})
		}

		wg.Wait()
		return nil
	},
}
