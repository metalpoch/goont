package storage

import (
	"database/sql"
	"fmt"
)

func createOltTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS olts (
			ip TEXT PRIMARY KEY,
			community TEXT NOT NULL,
			name TEXT,
			location TEXT,
			snmp_timeout INTEGER DEFAULT 60,
			snmp_retries INTEGER DEFAULT 3,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("create olts table: %w", err)
	}

	return nil
}

func createOntTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ont_measurements (
			time DATETIME,
			gpon_idx INTEGER NOT NULL,
			gpon_interface TEXT NOT NULL,
			ont_idx INTEGER NOT NULL,
			despt TEXT,
			serial_number TEXT,
			line_profile TEXT,
			control_ranging INTEGER,
			control_run_status INTEGER,
			temperature INTEGER,
			tx_power INTEGER,
			rx_power INTEGER,
			bytes_in TEXT, -- uint64
			bytes_out TEXT -- uint64
		)
	`)
	if err != nil {
		return fmt.Errorf("create ont_measurements table: %w", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ont_measurements_timestamp ON ont_measurements(time)
	`)
	if err != nil {
		return fmt.Errorf("create timestamp index: %w", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_gpon_measurements_port_idx ON ont_measurements(gpon_idx)
	`)
	if err != nil {
		return fmt.Errorf("create gpon index: %w", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_gpon_measurements_port_interface_name ON ont_measurements(gpon_interface)
	`)
	if err != nil {
		return fmt.Errorf("create gpon interface name: %w", err)
	}

	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_ont_measurements_port ON ont_measurements(gpon_idx, ont_idx)
	`)
	if err != nil {
		return fmt.Errorf("create ont index: %w", err)
	}

	return nil
}
