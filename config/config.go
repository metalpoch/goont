package config

import (
	"os"
	"strconv"
)

const (
	DefaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/goont?sslmode=disable"
	DefaultAddr        = "0.0.0.0:8080"
	DefaultSNMPConns   = 10
	DefaultMaxOLTs     = 32
)

type Config struct {
	DatabaseURL string
	Addr        string
	SNMPConns   int
	MaxOLTs     int
}

func Load() Config {
	cfg := Config{
		DatabaseURL: DefaultDatabaseURL,
		Addr:        DefaultAddr,
		SNMPConns:   DefaultSNMPConns,
		MaxOLTs:     DefaultMaxOLTs,
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

	return cfg
}
