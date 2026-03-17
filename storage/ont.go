package storage

import (
	"database/sql"
	"fmt"
	"goont/models"
	"time"
)

type OntClient struct {
	db *sql.DB
}

func NewOntDB(path string) (*OntClient, error) {
	dsn := path + "?" +
		"_pragma=journal_mode(WAL)&" +
		"_pragma=busy_timeout(5000)&" +
		"_pragma=synchronous=NORMAL&" +
		"_pragma=foreign_keys=ON"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := createOntTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &OntClient{db: db}, nil
}

func (o *OntClient) Close() {
	if o.db != nil {
		o.db.Close()
	}
}

func (o *OntClient) BatchInsertOntMeasurements(measurements []models.Ont) error {
	if len(measurements) == 0 {
		return nil
	}

	tx, err := o.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer tx.Rollback()

	stmt, err := tx.Prepare(`
			INSERT INTO ont_measurements (
				time, gpon_idx, gpon_interface, ont_idx, despt, serial_number,
				line_profile, control_ranging, control_run_status, temperature,
  			tx_power, rx_power, bytes_in, bytes_out
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}

	defer stmt.Close()

	for _, m := range measurements {
		_, err := stmt.Exec(
			m.Time, m.GponIdx, m.GponInterface, m.OntIdx, m.Despt, m.SerialNumber,
			m.LineProfName, m.ControlRanging, m.ControlRunStatus, m.Temperature,
			m.Tx, m.Rx, m.BytesIn, m.BytesOut,
		)
		if err != nil {
			return fmt.Errorf("insert measurement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (o *OntClient) GetMeasurements(startTime, endTime time.Time) ([]models.Ont, error) {
	rows, err := o.db.Query(`
        SELECT time, gpon_idx, gpon_interface, ont_idx, despt, serial_number,
               line_profile, control_ranging, control_run_status, temperature,
               tx_power, rx_power, bytes_in, bytes_out
        FROM ont_measurements
        WHERE time BETWEEN ? AND ?
        ORDER BY time DESC
    `, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("query measurements: %w", err)
	}
	defer rows.Close()

	var results []models.Ont
	for rows.Next() {
		var m models.Ont
		err := rows.Scan(
			&m.Time, &m.GponIdx, &m.GponInterface, &m.OntIdx, &m.Despt, &m.SerialNumber,
			&m.LineProfName, &m.ControlRanging, &m.ControlRunStatus, &m.Temperature,
			&m.Tx, &m.Rx, &m.BytesIn, &m.BytesOut,
		)
		if err != nil {
			return nil, fmt.Errorf("scan measurement: %w", err)
		}
		results = append(results, m)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if results == nil {
		return []models.Ont{}, nil
	}
	return results, nil
}

// GetMeasurementsByGponInRange returns measurements filtered by GPON index within a time range.
func (o *OntClient) GetMeasurementsByGpon(gponIdx int, startTime, endTime time.Time) ([]models.Ont, error) {
	rows, err := o.db.Query(`
        SELECT time, gpon_idx, gpon_interface, ont_idx, despt, serial_number,
               line_profile, control_ranging, control_run_status, temperature,
               tx_power, rx_power, bytes_in, bytes_out
        FROM ont_measurements
        WHERE time BETWEEN ? AND ? AND gpon_idx = ?
        ORDER BY time DESC
    `, startTime, endTime, gponIdx)
	if err != nil {
		return nil, fmt.Errorf("query measurements by gpon: %w", err)
	}
	defer rows.Close()

	var results []models.Ont
	for rows.Next() {
		var m models.Ont
		err := rows.Scan(
			&m.Time, &m.GponIdx, &m.GponInterface, &m.OntIdx, &m.Despt, &m.SerialNumber,
			&m.LineProfName, &m.ControlRanging, &m.ControlRunStatus, &m.Temperature,
			&m.Tx, &m.Rx, &m.BytesIn, &m.BytesOut,
		)
		if err != nil {
			return nil, fmt.Errorf("scan measurement: %w", err)
		}
		results = append(results, m)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if results == nil {
		return []models.Ont{}, nil
	}
	return results, nil
}

// GetMeasurementsByOntInRange returns measurements filtered by GPON index and ONT index within a time range.
func (o *OntClient) GetMeasurementsByOnt(gponIdx, ontIdx int, startTime, endTime time.Time) ([]models.Ont, error) {
	rows, err := o.db.Query(`
        SELECT time, gpon_idx, gpon_interface, ont_idx, despt, serial_number,
               line_profile, control_ranging, control_run_status, temperature,
               tx_power, rx_power, bytes_in, bytes_out
        FROM ont_measurements
        WHERE time BETWEEN ? AND ? AND gpon_idx = ? AND ont_idx = ?
        ORDER BY time DESC
    `, startTime, endTime, gponIdx, ontIdx)
	if err != nil {
		return nil, fmt.Errorf("query measurements by ont: %w", err)
	}
	defer rows.Close()

	var results []models.Ont
	for rows.Next() {
		var m models.Ont
		err := rows.Scan(
			&m.Time, &m.GponIdx, &m.GponInterface, &m.OntIdx, &m.Despt, &m.SerialNumber,
			&m.LineProfName, &m.ControlRanging, &m.ControlRunStatus, &m.Temperature,
			&m.Tx, &m.Rx, &m.BytesIn, &m.BytesOut,
		)
		if err != nil {
			return nil, fmt.Errorf("scan measurement: %w", err)
		}
		results = append(results, m)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if results == nil {
		return []models.Ont{}, nil
	}
	return results, nil
}
