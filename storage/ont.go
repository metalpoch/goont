package storage

import (
	"database/sql"
	"fmt"
	"goont/snmp"
	"strings"
	"sync"
	"time"
)

type OntClient struct {
	db *sql.DB
	mu sync.Mutex
}

func NewOntDB(path string) (*OntClient, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := createOntTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &OntClient{db: db}, nil
}

func (o *OntClient) Close() {
	o.db.Close()
}

func (o *OntClient) BatchInsertOntMeasurements(measurements []snmp.Ont) error {
	const maxRetries = 5
	initialDelay := 10 * time.Millisecond

	var lastErr error
	for retry := 0; retry < maxRetries; retry++ {
		if retry > 0 {
			delay := initialDelay * (1 << (retry - 1)) // exponential backoff
			time.Sleep(delay)
		}

		o.mu.Lock()

		tx, err := o.db.Begin()
		if err != nil {
			o.mu.Unlock()
			lastErr = fmt.Errorf("begin transaction: %w", err)
			if isDBLocked(err) {
				continue
			}
			return lastErr
		}

		stmt, err := tx.Prepare(`
			INSERT INTO ont_measurements (
				time, gpon_idx, gpon_interface, ont_idx, despt, serial_number,
				line_profile, control_ranging, control_run_status, temperature,
  			tx_power, rx_power, bytes_in, bytes_out
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			tx.Rollback()
			o.mu.Unlock()
			lastErr = fmt.Errorf("prepare statement: %w", err)
			if isDBLocked(err) {
				continue
			}
			return lastErr
		}

		allGood := true
		for _, m := range measurements {
			_, err := stmt.Exec(
				m.Time, m.GponIdx, m.GponInterface, m.OntIdx, m.Despt, m.SerialNumber,
				m.LineProfName, m.ControlRanging, m.ControlRunStatus, m.Temperature,
				m.Tx, m.Rx, m.BytesIn, m.BytesOut,
			)
			if err != nil {
				stmt.Close()
				tx.Rollback()
				o.mu.Unlock()
				lastErr = fmt.Errorf("insert measurement: %w", err)
				if isDBLocked(err) {
					allGood = false
					break
				}
				return lastErr
			}
		}

		if !allGood {
			stmt.Close()
			tx.Rollback()
			o.mu.Unlock()
			continue
		}

		stmt.Close()
		if err := tx.Commit(); err != nil {
			o.mu.Unlock()
			lastErr = fmt.Errorf("commit transaction: %w", err)
			if isDBLocked(err) {
				continue
			}
			return lastErr
		}

		o.mu.Unlock()
		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func isDBLocked(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY") ||
		strings.Contains(errStr, "database locked")
}
