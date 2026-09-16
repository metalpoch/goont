# API HTTP de GoONT

API de solo lectura sobre PostgreSQL/TimescaleDB. Todas las respuestas son JSON
(`Content-Type: application/json`). No requiere autenticación y expone CORS.

Base: `/api/v1`.

## Reglas de datos

### Intervalos de tráfico

Cada fila de los endpoints de tráfico representa **un intervalo entre dos
escaneos consecutivos** (no una medición instantánea).

- `bytes_in` / `bytes_out`: delta del contador acumulado SNMP en ese intervalo.
- `bps_in` / `bps_out`: velocidad media del intervalo, calculada en la BD con
  `LAG` (`delta * 8 / segundos`).

Un puerto GPON tiene una capacidad física de **2.5 Gbps**. Para que el contrato
solo entregue datos confiables, el backend descarta cualquier intervalo que no
sea plausible:

- delta negativo (`contador_actual < contador_previo`), típico de un reinicio de
  contador o *wrap*;
- `bps_in`/`bps_out > 2.5e9` (equivale a `bytes > 2.5e9 / 8 * segundos`), típico
  de saltos por cortes de energía.

En el endpoint por ONT (`/traffic/{ip}/{gpon}/{ont}`) rige además un tope por
**tasa contratada**: se extrae la velocidad del `line_profile` (p. ej.
`RES_HSI_Line_60M` → 60 Mbps) y se descarta todo intervalo que supere
`tasa * 1.5` (margen de ráfaga), acotado a 2.5 Gbps. Esto elimina los picos
irreales (300–600 Mbps en planes de 60 Mbps) provocados por reinicios o
re-registros de la ONT.

El `line_profile` es un nombre escrito a mano, así que el prefijo se normaliza:
`K` = miles, `M` = el número está en Mbps (`1000M` = 1 Gbps) y `G` se interpreta
como Gbps cuando el número es pequeño (`1G`, `2.5G`) o como Mbps cuando es
grande (`1000G` = 1000 Mbps). En los combos (`COMBO_Line_100M_4M`) se toma el
primer número, que es la tasa de **datos**; el segundo es telefonía y se ignora.
Si el perfil no contiene una cifra reconocible
(algunas ONT no la tienen), **no hay tope por plan** y se usa el límite del
puerto (2.5 Gbps); el tope por tasa es una heurística de seguridad, no una
fuente autoritativa. La tasa real solo se conoce en el OLT (perfil de línea),
por lo que leerla de SNMP queda como mejora futura.

Los intervalos inválidos **no se sirven** (se produce un *gap*): no se suman en
ninguna agregación ni se extrapolan. La regla se aplica en tiempo de consulta,
por lo que **también cubre datos históricos** almacenados antes de esta
validación.

> El endpoint del OLT (`/traffic/{ip}`) suma **solo** los intervalos válidos de
> sus puertos. Un puerto corrupto no contamina el total. Si en un timestamp
> todos los puertos son inválidos, la fila se omite. Como el OLT agrega varios
> puertos, su total sí puede superar 2.5 Gbps legítimamente.

### `null` significa "sin medición"

Los campos ópticos de una ONT (`temperature`, `olt_distance`, `tx_power`,
`rx_power`) devuelven `null` cuando la ONT no está operativa (`status != 1`,
p. ej. LOS) o cuando el SNMP no respondió. **Nunca** se sirven centinelas
(`2147483647`, `65535`). `null` no significa `0`: es ausencia de dato y la
gráfica debe dibujar un hueco.

### Conteo de ONTs y `count_unmeasured`

Las filas de `/traffic/{ip}` y `/traffic/{ip}/{gpon}` incluyen:

- `count_active`: ONT con `run_status = 1`.
- `count_inactive`: ONT con `run_status = 2`.
- `count_error`: ONT con `run_status = -1`.
- `count_unmeasured`: ONT del inventario que **no reportaron medición** en ese
  timestamp (dejaron de responder al SNMP).

Se cumple siempre:

```
count_active + count_inactive + count_error + count_unmeasured = inventario total
```

El total usado es el inventario actual de la tabla `onts` para ese OLT (o ese
puerto GPON). `count_unmeasured` se calcula como
`max(total - (active + inactive + error), 0)`, por lo que nunca es negativo.

### Parámetros de fecha

Los endpoints de tráfico exigen `initDate` y `endDate` en RFC3339:

```
?initDate=2026-09-04T00:00:00Z&endDate=2026-09-05T00:00:00Z
```

Se necesitan al menos **dos escaneos** en el rango para que exista tráfico
consultable (el primer intervalo no tiene predecesor y se omite).

## Endpoints

### `GET /api/v1/olt`

Lista de OLTs registrados.

```json
[
  {
    "ip": "192.168.1.1",
    "community": "public",
    "name": "OLT Centro",
    "location": "Sala 1",
    "created_at": "2026-09-01T12:00:00Z",
    "updated_at": "2026-09-01T12:00:00Z"
  }
]
```

### `GET /api/v1/olt/{ip}`

Detalle de un OLT. `404` si no existe.

### `GET /api/v1/olt/{ip}/ports`

Puertos GPON del OLT con su conteo de ONTs en inventario.

```json
[
  { "gpon_idx": 1, "gpon_interface": "GPON0/1/1", "ont_count": 42, "last_seen": "2026-09-05T12:00:00Z" }
]
```

### `GET /api/v1/olt/{ip}/onts?q=&gpon=`

Inventario de ONTs. `q` busca por descripción (DNI/RIF) o serial (`ILIKE`),
`gpon` filtra por puerto. Límite 500 resultados.

```json
[
  {
    "gpon_idx": 1,
    "ont_idx": 3,
    "desp": "V-12345678",
    "serial_number": "48575443A1B2C3D4",
    "line_profile": "profile-100M",
    "gpon_interface": "GPON0/1/1",
    "last_seen": "2026-09-05T12:00:00Z"
  }
]
```

### `GET /api/v1/traffic/{ip}`

Tráfico total del OLT: suma de los intervalos **válidos** de todos sus puertos
GPON, más el conteo de ONTs.

```json
[
  {
    "time": "2026-09-05T12:00:10Z",
    "bytes_in": 250000000,
    "bytes_out": 125000000,
    "bps_in": 200000000,
    "bps_out": 100000000,
    "count_active": 40,
    "count_inactive": 1,
    "count_error": 0,
    "count_unmeasured": 1
  }
]
```

### `GET /api/v1/traffic/{ip}/{gpon}`

Tráfico del puerto GPON (contadores IF-MIB del puerto) y conteo de sus ONTs.
`{gpon}` acepta el índice entero (admite valores sin signo de 32 bits).

```json
[
  {
    "time": "2026-09-05T12:00:10Z",
    "gpon_interface": "GPON0/1/1",
    "bytes_in": 250000000,
    "bytes_out": 125000000,
    "bps_in": 200000000,
    "bps_out": 100000000,
    "count_active": 40,
    "count_inactive": 1,
    "count_error": 0,
    "count_unmeasured": 1
  }
]
```

### `GET /api/v1/traffic/{ip}/{gpon}/{ont}`

Tráfico y señal óptica de una ONT. `status`: `1` working, `2` inactive/LOS,
`-1` error. Los campos ópticos son `null` sin medición.

```json
[
  {
    "time": "2026-09-05T12:00:10Z",
    "status": 1,
    "temperature": 45,
    "olt_distance": 1200,
    "tx_power": 2.0,
    "rx_power": -18.0,
    "bps_in": 200000000,
    "bps_out": 100000000,
    "bytes_in": 250000000,
    "bytes_out": 125000000,
    "desp": "V-12345678",
    "serial_number": "48575443A1B2C3D4",
    "plan": "profile-100M"
  }
]
```

ONT en LOS (obsérvese `null`, nunca `2147483647`):

```json
[
  {
    "time": "2026-09-05T12:05:10Z",
    "status": 2,
    "temperature": null,
    "olt_distance": null,
    "tx_power": null,
    "rx_power": null,
    "bps_in": 0,
    "bps_out": 0,
    "bytes_in": 0,
    "bytes_out": 0,
    "desp": "V-12345678",
    "serial_number": "48575443A1B2C3D4",
    "plan": "profile-100M"
  }
]
```

### `GET /api/v1/health`

Estado del servicio.

```json
{ "status": "ok" }
```

## Compatibilidad

Los cambios respecto al contrato anterior son **aditivos** para el consumidor,
salvo que ahora los campos ópticos pueden ser `null` (antes se servían
centinelas truncados). Se mantiene la misma versión `/api/v1`:

- nuevos campos: `count_unmeasured` en `/traffic/{ip}` y `/traffic/{ip}/{gpon}`;
- `temperature`, `olt_distance`, `tx_power`, `rx_power` son nullable y nunca
  contienen centinelas;
- los intervalos físicamente imposibles se omiten.

El consumidor debe tratar los campos ópticos como opcionales y dibujar un hueco
cuando sean `null`.
