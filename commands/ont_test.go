package commands

import (
	"testing"
	"time"

	"goont/config"
	"goont/models"
)

func TestScanLimits(t *testing.T) {
	cfg := config.Config{ScanTimeout: 15 * time.Second, ScanRetries: 2}

	tests := []struct {
		name        string
		olt         models.OLT
		wantTimeout time.Duration
		wantRetries int
	}{
		{
			name:        "acota un OLT con 60s y 3 reintentos",
			olt:         models.OLT{Timeout: 60, Retries: 3},
			wantTimeout: 15 * time.Second,
			wantRetries: 2,
		},
		{
			name:        "respeta valores menores por OLT",
			olt:         models.OLT{Timeout: 5, Retries: 1},
			wantTimeout: 5 * time.Second,
			wantRetries: 1,
		},
		{
			name:        "usa los globales si el OLT no define nada",
			olt:         models.OLT{Timeout: 0, Retries: 0},
			wantTimeout: 15 * time.Second,
			wantRetries: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTimeout, gotRetries := scanLimits(cfg, tt.olt)
			if gotTimeout != tt.wantTimeout {
				t.Fatalf("timeout: got %s, want %s", gotTimeout, tt.wantTimeout)
			}
			if gotRetries != tt.wantRetries {
				t.Fatalf("retries: got %d, want %d", gotRetries, tt.wantRetries)
			}
		})
	}
}
