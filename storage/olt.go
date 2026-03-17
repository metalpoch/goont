package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type OltClient struct {
	db *sql.DB
}

func NewOltDB(path string) (*OltClient, error) {
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

	if err := createOltTables(db); err != nil {
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return &OltClient{db: db}, nil
}

func (o *OltClient) Close() {
	o.db.Close()
}

func (o *OltClient) InsertOLT(olt OLT) error {
	timeoutSec := olt.Timeout
	if timeoutSec == 0 {
		timeoutSec = 60
	}
	retries := olt.Retries
	if retries == 0 {
		retries = 3
	}

	result, err := o.db.Exec(`
		INSERT INTO olts (ip, community, name, location, snmp_timeout, snmp_retries)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET
			community = excluded.community,
			name = excluded.name,
			location = excluded.location,
			snmp_timeout = excluded.snmp_timeout,
			snmp_retries = excluded.snmp_retries,
			updated_at = CURRENT_TIMESTAMP
	`, olt.IP, olt.Community, olt.Name, olt.Location, timeoutSec, retries)
	if err != nil {
		return fmt.Errorf("insert olt: %w", err)
	}

	_, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}
	return nil
}

func (o *OltClient) GetOLTByID(ip string) (*InfoOLT, error) {
	var olt InfoOLT
	err := o.db.QueryRow(`
		SELECT ip, community, name, location, created_at, updated_at
		FROM olts
		WHERE ip = ?
	`, ip).Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.CreatedAt, &olt.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query olt by ip: %w", err)
	}

	return &olt, nil
}

func (o *OltClient) GetInfoOLTs() ([]InfoOLT, error) {
	rows, err := o.db.Query(`
		SELECT ip, community, name, location, created_at, updated_at
		FROM olts
		ORDER BY ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query olts: %w", err)
	}
	defer rows.Close()

	var olts []InfoOLT
	for rows.Next() {
		var olt InfoOLT
		err := rows.Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.CreatedAt, &olt.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan olt: %w", err)
		}
		olts = append(olts, olt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return olts, nil
}

func (o *OltClient) GetOLTs() ([]OLT, error) {
	rows, err := o.db.Query(`
		SELECT 
		  ip,
		  community,
		  name,
		  location,
		  snmp_timeout,
		  snmp_retries,
		  created_at,
		  updated_at
		FROM olts
		ORDER BY ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query olts: %w", err)
	}
	defer rows.Close()
	var olts []OLT
	for rows.Next() {
		var olt OLT
		err := rows.Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.Timeout, &olt.Retries, &olt.CreatedAt, &olt.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan olt: %w", err)
		}
		olts = append(olts, olt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return olts, nil
}

func (o *OltClient) DeleteOLT(ip string) error {
	_, err := o.db.Exec("DELETE FROM olts WHERE ip = ?", ip)
	if err != nil {
		return fmt.Errorf("delete olt: %w", err)
	}
	return nil
}
