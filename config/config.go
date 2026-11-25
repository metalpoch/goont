package config

import (
	"os"
	"strconv"
	"time"
)

const (
	DefaultDatabaseURL  = "postgres://postgres:postgres@localhost:5432/goont?sslmode=disable"
	DefaultAddr         = "0.0.0.0:8080"
	DefaultSNMPConns    = 10
	DefaultMaxOLTs      = 32
	DefaultScanTimeout  = 15 * time.Second
	DefaultScanRetries  = 2
	DefaultScanDeadline = 4*time.Minute + 30*time.Second
)

type Config struct {
	DatabaseURL string
	Addr        string
	SNMPConns   int
	MaxOLTs     int
	// ScanTimeout y ScanRetries acotan las consultas SNMP del escaneo. Los
	// valores por OLT solo se usan si son menores: un OLT configurado con 60 s
	// no debe frenar el ciclo entero.
	ScanTimeout  time.Duration
	ScanRetries  int
	ScanDeadline time.Duration
}

func Load() Config {
	cfg := Config{
		DatabaseURL:  DefaultDatabaseURL,
		Addr:         DefaultAddr,
		SNMPConns:    DefaultSNMPConns,
		MaxOLTs:      DefaultMaxOLTs,
		ScanTimeout:  DefaultScanTimeout,
		ScanRetries:  DefaultScanRetries,
		ScanDeadline: DefaultScanDeadline,
	}

	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("GOONT_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("GOONT_SNMP_CONNS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.SNMPConns = n
		}
	}
	if v := os.Getenv("GOONT_MAX_OLTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxOLTs = n
		}
	}
	if v := os.Getenv("GOONT_SCAN_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ScanTimeout = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("GOONT_SCAN_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			cfg.ScanRetries = n
		}
	}
	if v := os.Getenv("GOONT_SCAN_DEADLINE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.ScanDeadline = time.Duration(n) * time.Second
		}
	}

	return cfg
}
