package traffic

import (
	"testing"
	"time"
)

func int32p(v int32) *int32 { return &v }

func TestMaxBytes(t *testing.T) {
	if got := MaxBytes(0); got != 0 {
		t.Fatalf("MaxBytes(0) = %d, want 0", got)
	}
	if got := MaxBytes(-time.Second); got != 0 {
		t.Fatalf("MaxBytes(negative) = %d, want 0", got)
	}

	dt := 10 * time.Second
	want := uint64(MaxGponBps / 8 * dt.Seconds())
	if got := MaxBytes(dt); got != want {
		t.Fatalf("MaxBytes(%v) = %d, want %d", dt, got, want)
	}
}

func TestPlausibleDelta(t *testing.T) {
	dt := 10 * time.Second
	max := MaxBytes(dt)

	tests := []struct {
		name      string
		prev, cur uint64
		want      bool
	}{
		{"delta cero", 1000, 1000, true},
		{"delta al limite", 0, max, true},
		{"delta sobre el limite", 0, max + 1, false},
		{"wrap decreciente", 5000, 1000, false},
		{"salto absurdo", 0, 10_000_000_000, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PlausibleDelta(tt.prev, tt.cur, dt); got != tt.want {
				t.Fatalf("PlausibleDelta(%d,%d,%v) = %v, want %v", tt.prev, tt.cur, dt, got, tt.want)
			}
		})
	}

	if PlausibleDelta(0, 100, 0) {
		t.Fatal("dt no positivo debe ser invalido")
	}
}

func TestPlausibleBps(t *testing.T) {
	if !PlausibleBps(MaxGponBps) {
		t.Fatal("el limite exacto debe ser plausible")
	}
	if PlausibleBps(MaxGponBps + 1) {
		t.Fatal("sobre el limite debe ser invalido")
	}
	if PlausibleBps(-1) {
		t.Fatal("negativo debe ser invalido")
	}
}

func TestIsSentinel(t *testing.T) {
	if !IsSentinel(SentinelInt32) || !IsSentinel(SentinelDistance) {
		t.Fatal("los centinelas deben detectarse")
	}
	if IsSentinel(0) || IsSentinel(-1800) {
		t.Fatal("valores validos no son centinelas")
	}
}

func TestValidOptical(t *testing.T) {
	working := int32p(1)
	los := int32p(2)
	errStatus := int32p(-1)

	tests := []struct {
		name   string
		status *int32
		value  *int32
		want   bool
	}{
		{"working con valor valido", working, int32p(45), true},
		{"working con centinela", working, int32p(SentinelInt32), false},
		{"working con centinela de distancia", working, int32p(SentinelDistance), false},
		{"LOS", los, int32p(45), false},
		{"error", errStatus, int32p(45), false},
		{"sin status", nil, int32p(45), false},
		{"sin valor", working, nil, false},
		{"valor negativo valido", working, int32p(-1800), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidOptical(tt.status, tt.value)
			if (got != nil) != tt.want {
				t.Fatalf("ValidOptical() = %v, want nil=%v", got, !tt.want)
			}
		})
	}
}
