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

func TestLineRateBps(t *testing.T) {
	tests := []struct {
		profile string
		want    float64
		ok      bool
	}{
		{"RES_HSI_Line_60M", 60e6, true},
		{"FTTH_1G", 1e9, true},
		{"plan_200Mbps", 200e6, true},
		{"line_1.5G", 1.5e9, true},
		{"line_512K", 512e3, true},
		{"plan_1000M", 1e9, true},
		{"plan_1000G", 1e9, true},
		{"plan_2.5G", 2.5e9, true},
		{"plan_10G", 1e10, true},
		{"COMBO_Line_100M_4M", 100e6, true},
		{"COMBO_Line_1G_4M", 1e9, true},
		{"COMBO_Line_400M_12M", 400e6, true},
		{"HG8145V5G", 0, false},
		{"GPON0/1", 0, false},
		{"RES_HSI_Line", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got, ok := LineRateBps(tt.profile)
			if ok != tt.ok {
				t.Fatalf("LineRateBps(%q) ok = %v, want %v", tt.profile, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("LineRateBps(%q) = %v, want %v", tt.profile, got, tt.want)
			}
		})
	}
}

func TestOntMaxBps(t *testing.T) {
	if got := OntMaxBps(0); got != MaxGponBps {
		t.Fatalf("OntMaxBps(0) = %v, want %v", got, MaxGponBps)
	}
	if got := OntMaxBps(60e6); got != 90e6 {
		t.Fatalf("OntMaxBps(60Mbps) = %v, want %v", got, 90e6)
	}
	if got := OntMaxBps(2e9); got != MaxGponBps {
		t.Fatalf("OntMaxBps(2Gbps) = %v, want %v", got, MaxGponBps)
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
