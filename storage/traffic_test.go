package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"goont/traffic"
)

func testStore(t *testing.T) *Store {
	t.Helper()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:postgres@localhost:5432/goont?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := Connect(ctx, url)
	if err != nil {
		t.Skipf("base de datos no disponible: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(pool.Close)

	return New(pool)
}

func cleanOlt(t *testing.T, s *Store, ip string) {
	t.Helper()
	cleanup := func() {
		ctx := context.Background()
		for _, q := range []string{
			`DELETE FROM ont_measurements WHERE olt_ip = $1`,
			`DELETE FROM gpon_measurements WHERE olt_ip = $1`,
			`DELETE FROM onts WHERE olt_ip = $1`,
		} {
			if _, err := s.pool.Exec(ctx, q, ip); err != nil {
				t.Fatalf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
}

func insertGpon(t *testing.T, s *Store, ip string, gponIdx uint64, tm time.Time, in, out uint64) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO gpon_measurements (time, olt_ip, gpon_idx, bytes_in, bytes_out) VALUES ($1,$2,$3,$4,$5)`,
		tm, ip, int64(gponIdx), int64(in), int64(out))
	if err != nil {
		t.Fatalf("insert gpon: %v", err)
	}
}

func insertOnt(t *testing.T, s *Store, ip string, gponIdx uint64, ontIdx int, tm time.Time,
	status, dist, temp, tx, rx *int32, in, out uint64) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO ont_measurements
		 (time, olt_ip, gpon_idx, ont_idx, run_status, olt_distance, temperature, tx_power, rx_power, bytes_in, bytes_out)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		tm, ip, int64(gponIdx), ontIdx, status, dist, temp, tx, rx, int64(in), int64(out))
	if err != nil {
		t.Fatalf("insert ont: %v", err)
	}
}

func insertOntInventory(t *testing.T, s *Store, ip string, gponIdx uint64, ontIdx int) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO onts (olt_ip, gpon_idx, ont_idx, serial_number, description, line_profile, gpon_interface, last_seen)
		 VALUES ($1,$2,$3,'','','','',now()) ON CONFLICT DO NOTHING`,
		ip, int64(gponIdx), ontIdx)
	if err != nil {
		t.Fatalf("insert inventory: %v", err)
	}
}

func i32(v int32) *int32 { return &v }

func baseTimes() (t0, t1, t2, t3, t4 time.Time) {
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	return base, base.Add(10 * time.Second), base.Add(20 * time.Second), base.Add(30 * time.Second), base.Add(40 * time.Second)
}

// Criterio 1 y 4: un pico falso y un wrap se omiten; los bps validos no superan 2.5 Gbps.
func TestGponTraffic_PeakAndWrapAreOmitted(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.1"
	cleanOlt(t, s, ip)

	t0, t1, t2, t3, t4 := baseTimes()

	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 250_000_000, 250_000_000)       // valido (~200 Mbps)
	insertGpon(t, s, ip, 1, t2, 10_000_000_000, 10_000_000_000) // pico falso (=> ~7.8 Gbps)
	insertGpon(t, s, ip, 1, t3, 500_000_000, 500_000_000)       // wrap (decrece)
	insertGpon(t, s, ip, 1, t4, 750_000_000, 750_000_000)       // valido (~200 Mbps)

	got, err := s.GponTrafficData(context.Background(), ip, 1, t0, t4)
	if err != nil {
		t.Fatalf("GponTrafficData: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("se esperaban 2 intervalos validos, got %d: %+v", len(got), got)
	}
	if !got[0].Time.Equal(t1) || !got[1].Time.Equal(t4) {
		t.Fatalf("intervalos inesperados: %v, %v", got[0].Time, got[1].Time)
	}
	for _, m := range got {
		if m.BpsIn > traffic.MaxGponBps || m.BpsOut > traffic.MaxGponBps {
			t.Fatalf("bps sobre el limite servido: %+v", m)
		}
	}
}

// Criterio 2: el OLT solo suma intervalos de puerto validos.
func TestOltTraffic_IgnoresInvalidPorts(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.2"
	cleanOlt(t, s, ip)

	t0, t1, _, _, _ := baseTimes()

	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 250_000_000, 250_000_000) // valido

	insertGpon(t, s, ip, 2, t0, 0, 0)
	insertGpon(t, s, ip, 2, t1, 10_000_000_000, 10_000_000_000) // invalido

	insertOntInventory(t, s, ip, 1, 1)

	got, err := s.OltTraffic(context.Background(), ip, t0, t1)
	if err != nil {
		t.Fatalf("OltTraffic: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("se esperaba 1 intervalo, got %d", len(got))
	}
	if got[0].BytesIn != 250_000_000 || got[0].BytesOut != 250_000_000 {
		t.Fatalf("el OLT sumo el puerto invalido: %+v", got[0])
	}
	if got[0].BpsIn > traffic.MaxGponBps || got[0].BpsOut > traffic.MaxGponBps {
		t.Fatalf("bps del OLT sobre el limite: %+v", got[0])
	}
}

// Criterio 2: si todos los puertos de un timestamp son invalidos, la fila se omite.
func TestOltTraffic_AllPortsInvalidOmitted(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.3"
	cleanOlt(t, s, ip)

	t0, t1, t2, _, _ := baseTimes()

	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 2, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 10_000_000_000, 0) // invalido
	insertGpon(t, s, ip, 2, t1, 0, 10_000_000_000) // invalido
	insertGpon(t, s, ip, 1, t2, 10_000_000_000+250_000_000, 250_000_000)
	insertGpon(t, s, ip, 2, t2, 250_000_000, 10_000_000_000+250_000_000)

	got, err := s.OltTraffic(context.Background(), ip, t0, t2)
	if err != nil {
		t.Fatalf("OltTraffic: %v", err)
	}
	for _, m := range got {
		if m.Time.Equal(t1) {
			t.Fatalf("el timestamp con todos los puertos invalidos no debe aparecer: %+v", m)
		}
	}
	if len(got) != 1 || !got[0].Time.Equal(t2) {
		t.Fatalf("se esperaba solo t2, got %+v", got)
	}
}

// Criterio 3: la optica nunca devuelve centinelas; es null si la ONT no esta working.
func TestOntTraffic_OpticsNeverServeSentinels(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.4"
	cleanOlt(t, s, ip)

	t0, t1, _, _, _ := baseTimes()

	insertOnt(t, s, ip, 1, 1, t0, i32(1), i32(1200), i32(45), i32(200), i32(-1800), 0, 0)
	insertOnt(t, s, ip, 1, 1, t1, i32(1), i32(1200), i32(45), i32(200), i32(-1800), 250_000_000, 250_000_000)

	insertOnt(t, s, ip, 1, 2, t0, i32(2), i32(traffic.SentinelInt32), i32(traffic.SentinelInt32),
		i32(traffic.SentinelInt32), i32(traffic.SentinelInt32), 0, 0)
	insertOnt(t, s, ip, 1, 2, t1, i32(2), i32(traffic.SentinelInt32), i32(traffic.SentinelInt32),
		i32(traffic.SentinelInt32), i32(traffic.SentinelInt32), 250_000_000, 250_000_000)

	got, err := s.OntTraffic(context.Background(), ip, 1, 2, t0, t1)
	if err != nil {
		t.Fatalf("OntTraffic: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("se esperaba 1 intervalo, got %d", len(got))
	}
	m := got[0]
	if m.Temperature != nil || m.OltDistance != nil || m.Tx != nil || m.Rx != nil {
		t.Fatalf("la ONT en LOS no debe servir optica: %+v", m)
	}

	working, err := s.OntTraffic(context.Background(), ip, 1, 1, t0, t1)
	if err != nil {
		t.Fatalf("OntTraffic working: %v", err)
	}
	if len(working) != 1 {
		t.Fatalf("se esperaba 1 intervalo working, got %d", len(working))
	}
	w := working[0]
	if w.Temperature == nil || *w.Temperature != 45 {
		t.Fatalf("temperature working = %v", w.Temperature)
	}
	if w.OltDistance == nil || *w.OltDistance != 1200 {
		t.Fatalf("olt_distance working = %v", w.OltDistance)
	}
	if w.Tx == nil || *w.Tx != 2.0 {
		t.Fatalf("tx working = %v", w.Tx)
	}
	if w.Rx == nil || *w.Rx != -18.0 {
		t.Fatalf("rx working = %v", w.Rx)
	}
}

// Criterio 4: count_unmeasured completa el inventario cuando una ONT no responde.
func TestTraffic_CountUnmeasured(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.5"
	cleanOlt(t, s, ip)

	t0, t1, _, _, _ := baseTimes()

	insertOntInventory(t, s, ip, 1, 1)
	insertOntInventory(t, s, ip, 1, 2)
	insertOntInventory(t, s, ip, 1, 3) // no respondera

	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 250_000_000, 250_000_000)

	insertOnt(t, s, ip, 1, 1, t1, i32(1), nil, nil, nil, nil, 0, 0) // active
	insertOnt(t, s, ip, 1, 2, t1, i32(2), nil, nil, nil, nil, 0, 0) // inactive

	port, err := s.GponTrafficData(context.Background(), ip, 1, t0, t1)
	if err != nil {
		t.Fatalf("GponTrafficData: %v", err)
	}
	if len(port) != 1 {
		t.Fatalf("se esperaba 1 intervalo, got %d", len(port))
	}
	if port[0].CountActive != 1 || port[0].CountInactive != 1 || port[0].CountError != 0 || port[0].CountUnmeasured != 1 {
		t.Fatalf("conteos de puerto inesperados: %+v", port[0])
	}

	olt, err := s.OltTraffic(context.Background(), ip, t0, t1)
	if err != nil {
		t.Fatalf("OltTraffic: %v", err)
	}
	if len(olt) != 1 {
		t.Fatalf("se esperaba 1 intervalo de OLT, got %d", len(olt))
	}
	sum := olt[0].CountActive + olt[0].CountInactive + olt[0].CountError + olt[0].CountUnmeasured
	if sum != 3 {
		t.Fatalf("la suma de conteos (%d) no cuadra con el inventario (3): %+v", sum, olt[0])
	}
	if olt[0].CountUnmeasured < 0 {
		t.Fatalf("count_unmeasured no puede ser negativo: %+v", olt[0])
	}
}

// count_unmeasured nunca es negativo aunque haya mas mediciones que inventario.
func TestGponTraffic_UnmeasuredNeverNegative(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.6"
	cleanOlt(t, s, ip)

	t0, t1, _, _, _ := baseTimes()

	insertOntInventory(t, s, ip, 1, 1)
	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 250_000_000, 250_000_000)
	insertOnt(t, s, ip, 1, 1, t1, i32(1), nil, nil, nil, nil, 0, 0)
	insertOnt(t, s, ip, 1, 2, t1, i32(1), nil, nil, nil, nil, 0, 0)

	port, err := s.GponTrafficData(context.Background(), ip, 1, t0, t1)
	if err != nil {
		t.Fatalf("GponTrafficData: %v", err)
	}
	if len(port) != 1 {
		t.Fatalf("se esperaba 1 intervalo, got %d", len(port))
	}
	if port[0].CountUnmeasured != 0 {
		t.Fatalf("count_unmeasured debe acotarse a 0, got %d", port[0].CountUnmeasured)
	}
}
