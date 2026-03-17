package commands

import (
	"fmt"
	"goont/models"
	"goont/snmp"
	"goont/storage"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func getAllInfoOLTS() ([]models.InfoOLT, error) {
	dir, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("Error al intentar obtener la ruta del ejecutable")
	}

	dbDir := filepath.Join(filepath.Dir(dir), "olt.db")
	client, err := storage.NewOltDB(dbDir)
	if err != nil {
		return nil, fmt.Errorf("Error al intentar acceder a la base de datos: %v", err)
	}

	defer client.Close()

	olts, err := client.GetInfoOLTs()
	if err != nil {
		return nil, fmt.Errorf("Error al intentar obtener todos los OLT: %v", err)
	}

	return olts, nil
}

func getAllOLTS(dir string) ([]models.OLT, error) {
	dbDir := filepath.Join(filepath.Dir(dir), "olt.db")
	client, err := storage.NewOltDB(dbDir)
	if err != nil {
		return nil, fmt.Errorf("Error al intentar acceder a la base de datos: %v", err)
	}

	defer client.Close()

	olts, err := client.GetOLTs()
	if err != nil {
		return nil, fmt.Errorf("Error al intentar obtener todos los OLT: %v", err)
	}

	return olts, nil
}

func ontScanner(olt *snmp.Snmp) []models.Ont {
	gponIfname, err := olt.IfNames()
	if err != nil {
		panic(err)
	}

	now := time.Now()
	sem := make(chan struct{}, 20)
	ontsBuffer := make(chan []models.Ont, len(gponIfname))

	var wg sync.WaitGroup

	for _, g := range gponIfname {
		sem <- struct{}{}

		wg.Go(func() {
			defer func() { <-sem }()

			allOnt, err := olt.OntQuery(g)
			if err != nil {
				log.Printf("error al ejecutar las consultas a los ont de %s del puerto %d (%s): %v\n", olt.IP, g.Idx, g.IfName, err)
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
