// Package traffic concentra las reglas de plausibilidad física del tráfico
// GPON y la sanitización de métricas ópticas de las ONT.
//
// Un puerto GPON no puede superar 2.5 Gbps, por lo que cualquier intervalo
// cuyo delta de contador implique una velocidad mayor es un artefacto (p.ej.
// un corte de energía) y debe descartarse. Las ONT en LOS/error o sin
// respuesta SNMP devuelven centinelas int32 (2147483647) que nunca deben
// servirse al consumidor.
package traffic

import "time"

// MaxGponBps es la capacidad física de un puerto GPON (2.5 Gbps).
const MaxGponBps float64 = 2_500_000_000

// SentinelInt32 es el valor centinela que Huawei usa para "sin medición".
const SentinelInt32 int32 = 2147483647

// SentinelDistance es otro centinela observado en el control de ranging.
const SentinelDistance int32 = 65535

// maxRepresentableBytes evita desbordar uint64 al calcular el máximo de bytes.
const maxRepresentableBytes = ^uint64(0)

// MaxBytes devuelve el máximo de octetos plausibles transferidos en dt a la
// velocidad del puerto GPON. Es 0 si dt no es positivo.
func MaxBytes(dt time.Duration) uint64 {
	if dt <= 0 {
		return 0
	}
	max := MaxGponBps / 8 * dt.Seconds()
	if max >= float64(maxRepresentableBytes) {
		return maxRepresentableBytes
	}
	return uint64(max)
}

// PlausibleDelta indica si el delta de un contador acumulado entre dos
// escaneos separados por dt es físicamente posible. Los decrementos
// (reinicios de contador) y los saltos por encima de la capacidad del puerto
// se consideran inválidos.
func PlausibleDelta(prev, cur uint64, dt time.Duration) bool {
	if dt <= 0 || cur < prev {
		return false
	}
	return cur-prev <= MaxBytes(dt)
}

// PlausibleBps indica si una velocidad calculada cabe dentro del puerto.
func PlausibleBps(bps float64) bool {
	return bps >= 0 && bps <= MaxGponBps
}

// IsSentinel indica si un valor entero es un centinela de "sin medición".
func IsSentinel(v int32) bool {
	return v == SentinelInt32 || v == SentinelDistance
}

// ValidOptical devuelve value solo si la ONT está operativa (status == 1) y el
// valor no es un centinela. En cualquier otro caso devuelve nil, que se
// serializa como null en JSON y significa "sin medición".
func ValidOptical(status *int32, value *int32) *int32 {
	if status == nil || *status != 1 || value == nil || IsSentinel(*value) {
		return nil
	}
	return value
}
