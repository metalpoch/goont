package config

import (
	"testing"
	"time"
)

func TestLoadScanDefaults(t *testing.T) {
	t.Setenv("GOONT_SCAN_TIMEOUT", "")
	t.Setenv("GOONT_SCAN_RETRIES", "")
	t.Setenv("GOONT_SCAN_DEADLINE", "")

	cfg := Load()

	if cfg.ScanTimeout != 15*time.Second {
		t.Fatalf("ScanTimeout por defecto: got %s, want 15s", cfg.ScanTimeout)
	}
	if cfg.ScanRetries != 2 {
		t.Fatalf("ScanRetries por defecto: got %d, want 2", cfg.ScanRetries)
	}
	if cfg.ScanDeadline != 4*time.Minute+30*time.Second {
		t.Fatalf("ScanDeadline por defecto: got %s, want 4m30s", cfg.ScanDeadline)
	}
}

func TestLoadScanEnv(t *testing.T) {
	t.Setenv("GOONT_SCAN_TIMEOUT", "8")
	t.Setenv("GOONT_SCAN_RETRIES", "1")
	t.Setenv("GOONT_SCAN_DEADLINE", "120")

	cfg := Load()

	if cfg.ScanTimeout != 8*time.Second {
		t.Fatalf("ScanTimeout: got %s, want 8s", cfg.ScanTimeout)
	}
	if cfg.ScanRetries != 1 {
		t.Fatalf("ScanRetries: got %d, want 1", cfg.ScanRetries)
	}
	if cfg.ScanDeadline != 120*time.Second {
		t.Fatalf("ScanDeadline: got %s, want 2m", cfg.ScanDeadline)
	}
}

func TestLoadScanEnvInvalidKeepsDefaults(t *testing.T) {
	t.Setenv("GOONT_SCAN_TIMEOUT", "no-es-numero")
	t.Setenv("GOONT_SCAN_RETRIES", "-1")
	t.Setenv("GOONT_SCAN_DEADLINE", "0")

	cfg := Load()

	if cfg.ScanTimeout != DefaultScanTimeout {
		t.Fatalf("ScanTimeout inválido: got %s, want %s", cfg.ScanTimeout, DefaultScanTimeout)
	}
	if cfg.ScanRetries != DefaultScanRetries {
		t.Fatalf("ScanRetries inválido: got %d, want %d", cfg.ScanRetries, DefaultScanRetries)
	}
	if cfg.ScanDeadline != DefaultScanDeadline {
		t.Fatalf("ScanDeadline inválido: got %s, want %s", cfg.ScanDeadline, DefaultScanDeadline)
	}
}
