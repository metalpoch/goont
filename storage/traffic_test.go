package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"goont/models"
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

// setGponCounts simula los contadores que el escaneo persiste junto al sample.
func setGponCounts(t *testing.T, s *Store, ip string, gponIdx uint64, tm time.Time, active, inactive, failed int) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(),
		`UPDATE gpon_measurements
		 SET count_active = $1, count_inactive = $2, count_error = $3
		 WHERE olt_ip = $4 AND gpon_idx = $5 AND time = $6`,
		active, inactive, failed, ip, int64(gponIdx), tm)
	if err != nil {
		t.Fatalf("set gpon counts: %v", err)
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
		 VALUES ($1,$2,$3,'','','', 'GPON0/1/' || $2::bigint, now()) ON CONFLICT DO NOTHING`,
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
	setGponCounts(t, s, ip, 1, t1, 1, 1, 0)

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

// Criterio 4: un pico por encima del plan contratado (60 Mbps) se omite.
func TestOntTraffic_PlanPeakOmitted(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.7"
	cleanOlt(t, s, ip)

	t0, t1, t2, t3, _ := baseTimes()

	_, err := s.pool.Exec(context.Background(),
		`INSERT INTO ont_measurements
		 (time, olt_ip, gpon_idx, ont_idx, line_profile, run_status, bytes_in, bytes_out)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		t0, ip, int64(1), 1, "RES_HSI_Line_60M", int32(1), int64(0), int64(0))
	if err != nil {
		t.Fatalf("insert ont profile: %v", err)
	}
	insertOntProfile := func(tm time.Time, in, out uint64) {
		t.Helper()
		_, err := s.pool.Exec(context.Background(),
			`INSERT INTO ont_measurements
			 (time, olt_ip, gpon_idx, ont_idx, line_profile, run_status, bytes_in, bytes_out)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			tm, ip, int64(1), 1, "RES_HSI_Line_60M", int32(1), int64(in), int64(out))
		if err != nil {
			t.Fatalf("insert ont profile: %v", err)
		}
	}

	insertOntProfile(t1, 62_500_000, 62_500_000)   // ~50 Mbps: valido
	insertOntProfile(t2, 788_500_000, 788_500_000) // pico falso ~581 Mbps
	insertOntProfile(t3, 851_000_000, 851_000_000) // ~50 Mbps: valido

	got, err := s.OntTraffic(context.Background(), ip, 1, 1, t0, t3)
	if err != nil {
		t.Fatalf("OntTraffic: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("se esperaban 2 intervalos validos, got %d: %+v", len(got), got)
	}
	for _, m := range got {
		if m.Time.Equal(t2) {
			t.Fatalf("el pico por encima del plan no debe servirse: %+v", m)
		}
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
	setGponCounts(t, s, ip, 1, t1, 2, 0, 0)

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

// El escaneo persiste los conteos del puerto junto al sample; la consulta de
// trafico ya no vuelve a leer ont_measurements.
func TestInsertGponMeasurements_PersistsOntCounts(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.8"
	cleanOlt(t, s, ip)

	tm := time.Now().UTC().Truncate(time.Second)

	onts := []models.Ont{
		{Time: tm, GponIdx: 1, OntIdx: 1, ControlRunStatus: i32(1)},
		{Time: tm, GponIdx: 1, OntIdx: 2, ControlRunStatus: i32(1)},
		{Time: tm, GponIdx: 1, OntIdx: 3, ControlRunStatus: i32(2)},
		{Time: tm, GponIdx: 1, OntIdx: 4, ControlRunStatus: i32(-1)},
		{Time: tm, GponIdx: 1, OntIdx: 5}, // sin run_status: no suma a ningun conteo
		{Time: tm, GponIdx: 2, OntIdx: 1, ControlRunStatus: i32(1)},
	}
	samples := []models.GponSample{
		{Time: tm, GponIdx: 1, BytesIn: 100, BytesOut: 50},
		{Time: tm, GponIdx: 2, BytesIn: 200, BytesOut: 100},
	}

	if err := s.InsertGponMeasurements(context.Background(), ip, samples, onts); err != nil {
		t.Fatalf("InsertGponMeasurements: %v", err)
	}

	check := func(gponIdx uint64, wantActive, wantInactive, wantError int) {
		t.Helper()
		var active, inactive, failed int
		err := s.pool.QueryRow(context.Background(),
			`SELECT count_active, count_inactive, count_error FROM gpon_measurements
			 WHERE olt_ip = $1 AND gpon_idx = $2 AND time = $3`,
			ip, int64(gponIdx), tm).Scan(&active, &inactive, &failed)
		if err != nil {
			t.Fatalf("query counts gpon %d: %v", gponIdx, err)
		}
		if active != wantActive || inactive != wantInactive || failed != wantError {
			t.Fatalf("gpon %d counts = (%d,%d,%d), se esperaba (%d,%d,%d)",
				gponIdx, active, inactive, failed, wantActive, wantInactive, wantError)
		}
	}

	check(1, 2, 1, 1)
	check(2, 1, 0, 0)
}

// El endpoint agregado devuelve, para cada puerto, exactamente la misma serie
// que el endpoint individual.
func TestPortsTraffic_MatchesSinglePort(t *testing.T) {
	s := testStore(t)
	const ip = "10.10.0.9"
	cleanOlt(t, s, ip)

	t0, t1, t2, _, _ := baseTimes()

	insertOntInventory(t, s, ip, 1, 1)
	insertOntInventory(t, s, ip, 1, 2)
	insertOntInventory(t, s, ip, 2, 1)

	insertGpon(t, s, ip, 1, t0, 0, 0)
	insertGpon(t, s, ip, 1, t1, 250_000_000, 250_000_000)
	insertGpon(t, s, ip, 1, t2, 500_000_000, 500_000_000)
	setGponCounts(t, s, ip, 1, t1, 1, 1, 0)
	setGponCounts(t, s, ip, 1, t2, 2, 0, 0)

	insertGpon(t, s, ip, 2, t0, 0, 0)
	insertGpon(t, s, ip, 2, t1, 125_000_000, 125_000_000)
	setGponCounts(t, s, ip, 2, t1, 1, 0, 0)

	ports, err := s.PortsTraffic(context.Background(), ip, t0, t2)
	if err != nil {
		t.Fatalf("PortsTraffic: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("se esperaban 2 puertos, got %d: %+v", len(ports), ports)
	}

	single, err := s.GponTrafficData(context.Background(), ip, 1, t0, t2)
	if err != nil {
		t.Fatalf("GponTrafficData: %v", err)
	}
	if len(ports[0].Measurements) != len(single) {
		t.Fatalf("puerto 1: %d intervalos en el agregado, %d en el individual",
			len(ports[0].Measurements), len(single))
	}
	for i := range single {
		got, want := ports[0].Measurements[i], single[i]
		if !got.Time.Equal(want.Time) || got.BytesIn != want.BytesIn || got.BytesOut != want.BytesOut ||
			got.BpsIn != want.BpsIn || got.BpsOut != want.BpsOut ||
			got.CountActive != want.CountActive || got.CountInactive != want.CountInactive ||
			got.CountError != want.CountError || got.CountUnmeasured != want.CountUnmeasured {
			t.Fatalf("intervalo %d distinto:\n agregado=%+v\n individual=%+v", i, got, want)
		}
	}
	if ports[0].GponInterface != "GPON0/1/1" {
		t.Fatalf("interfaz del puerto 1 = %q", ports[0].GponInterface)
	}
}

// Aplicar Migrate dos veces no debe fallar ni duplicar columnas o indices.
func TestMigrate_Idempotent(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := Migrate(ctx, s.pool); err != nil {
			t.Fatalf("Migrate #%d: %v", i+2, err)
		}
	}

	var columns int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'gpon_measurements'
		  AND column_name IN ('count_active', 'count_inactive', 'count_error')
	`).Scan(&columns)
	if err != nil {
		t.Fatalf("lookup columns: %v", err)
	}
	if columns != 3 {
		t.Fatalf("se esperaban 3 columnas de conteo, hay %d", columns)
	}

	var indexes int
	err = s.pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_indexes
		WHERE indexname = 'idx_gpon_measurements_time'
	`).Scan(&indexes)
	if err != nil {
		t.Fatalf("lookup indexes: %v", err)
	}
	if indexes != 1 {
		t.Fatalf("se esperaba el indice idx_gpon_measurements_time, hay %d", indexes)
	}

	// ont_measurements ya no necesita un indice (olt_ip, time): comprobamos que
	// Migrate no lo cree (bloquearia las escrituras durante minutos en tablas
	// grandes sin que ninguna consulta lo use).
	var unwanted int
	err = s.pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_indexes WHERE indexname = 'idx_ont_measurements_time'
	`).Scan(&unwanted)
	if err != nil {
		t.Fatalf("lookup unwanted index: %v", err)
	}
	if unwanted != 0 {
		t.Fatalf("no deberia existir idx_ont_measurements_time")
	}
}
