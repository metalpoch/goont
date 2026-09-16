// Package traffic concentra las reglas de plausibilidad física del tráfico
// GPON y la sanitización de métricas ópticas de las ONT.
//
// Un puerto GPON no puede superar 2.5 Gbps, por lo que cualquier intervalo
// cuyo delta de contador implique una velocidad mayor es un artefacto (p.ej.
// un corte de energía) y debe descartarse. Las ONT en LOS/error o sin
// respuesta SNMP devuelven centinelas int32 (2147483647) que nunca deben
// servirse al consumidor.
package traffic

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaxGponBps es la capacidad física de un puerto GPON (2.5 Gbps).
const MaxGponBps float64 = 2_500_000_000

// BurstFactor es el margen sobre la tasa contratada que se tolera como ráfaga
// legítima. Por encima de este múltiplo el intervalo se considera un artefacto
// (contador reiniciado, ONT re-registrada, lectura corrupta).
const BurstFactor float64 = 1.5

// lineRateRe captura la tasa de un perfil de línea Huawei, p.ej.
// "RES_HSI_Line_60M" -> 60 M, "FTTH_1G" -> 1 G. Exige que el número no esté
// pegado a otro alfanumérico (evita falsos positivos como "V5G").
var lineRateRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d+(?:[.,]\d+)?)\s*([kmg])(?:bps)?`)

// LineRateBps extrae la velocidad contratada en bps del nombre de perfil de
// línea. El nombre lo escribe una persona, así que el prefijo se normaliza:
//
//   - K: miles ("512K" -> 512 kbps).
//   - M: el número ya está en Mbps ("60M" -> 60 Mbps, "1000M" -> 1 Gbps).
//   - G: el número está en Mbps cuando es grande ("1000G" -> 1000 Mbps) y en
//     Gbps cuando es pequeño ("1G" -> 1 Gbps, "2.5G" -> 2.5 Gbps).
//
// Devuelve ok=false si no hay una cifra reconocible. La tasa es solo un techo
// heurístico: el llamador debe acotarla con OntMaxBps y, si no hay tasa, usar
// la capacidad del puerto.
//
// En los productos combo ("COMBO_Line_100M_4M") el primer número es la tasa de
// datos y el segundo la telefonía, por eso se toma el primero que aparezca.
func LineRateBps(profile string) (float64, bool) {
	m := lineRateRe.FindStringSubmatch(profile)
	if m == nil {
		return 0, false
	}

	value, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	if err != nil || value <= 0 {
		return 0, false
	}

	var mbps float64
	switch strings.ToLower(m[2]) {
	case "k":
		mbps = value / 1e3
	case "m":
		mbps = value
	case "g":
		if value >= 100 {
			mbps = value
		} else {
			mbps = value * 1e3
		}
	default:
		return 0, false
	}

	return mbps * 1e6, true
}

// OntMaxBps devuelve el máximo de bps plausible para una ONT de tasa rateBps:
// la tasa contratada ampliada por BurstFactor, acotada a la capacidad del
// puerto. Si la tasa no es positiva usa el límite del puerto.
func OntMaxBps(rateBps float64) float64 {
	if rateBps <= 0 {
		return MaxGponBps
	}
	max := rateBps * BurstFactor
	if max > MaxGponBps {
		return MaxGponBps
	}
	return max
}

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
