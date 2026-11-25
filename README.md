# GoONT - Herramienta de Gestión de OLT/ONT

GoONT es una herramienta de línea de comandos (CLI) escrita en Go para la gestión y monitoreo de equipos OLT (Optical Line Terminal) y ONT (Optical Network Terminal) en redes GPON mediante protocolo SNMP.

## Características

- **Gestión de OLTs**: Agregar, listar y eliminar OLTs de la base de datos
- **Escaneo de ONTs**: Obtención automática de información de ONTs conectados a cada OLT
- **Tráfico de puertos GPON**: Contadores del puerto vía MIB PON de Huawei (`hwGponOltEthernetStatisticReceivedBytes`/`SendBytes`), sin sumar ONTs
- **Almacenamiento**: PostgreSQL con TimescaleDB (hypertables + compresión automática)
- **Consultas SNMP**: Soporte para SNMP v2c con pool de conexiones reutilizables por OLT
- **Concurrente**: Escaneo paralelo de múltiples OLTs y puertos GPON

## Instalación

### Requisitos previos
- Go 1.25.6 o superior
- PostgreSQL con la extensión TimescaleDB (ej: contenedor `timescale/timescaledb`)

### Desde código fuente
```bash
git clone https://github.com/metalpoch/goont
cd goont
go build -o goont ./cmd/cli
```

## Uso

### Comandos disponibles

```bash
# Gestionar OLTs
goont olt list          # Listar todos los OLTs registrados
goont olt add           # Agregar un nuevo OLT
goont olt remove        # Eliminar un OLT

# Escanear ONTs
goont ont scan          # Escanear ONTs en todos los OLTs registrados
```

### Agregar un OLT
```bash
goont olt add --ip 192.168.1.1 --community public --timeout 60 --retries 3
```

### Escanear ONTs
```bash
goont ont scan
```

## Estructura del Proyecto

```
goont/
├── cmd/
│   ├── cli/main.go      # Entrada de la CLI
│   └── server/main.go   # Entrada del servidor HTTP
├── commands/            # Implementación de comandos CLI
├── config/              # Configuración por variables de entorno
├── handlers/            # Handlers HTTP de la API
├── middleware/          # Logging, RecoverPanic, CORS
├── models/              # Tipos compartidos
├── snmp/                # Cliente SNMP (pool de conexiones, OIDs Huawei GPON)
└── storage/             # PostgreSQL/TimescaleDB (migraciones, escritura masiva, consultas)
```

## Base de Datos

PostgreSQL + TimescaleDB. Las tablas se crean automáticamente al primer arranque:

- `olts`: registro de OLTs (IP, community, timeouts)
- `onts`: identidad de cada ONT visto (IP, puerto GPON, índice, serial)
- `ont_measurements`: hypertable con mediciones por ONT (contadores, señal, estado)
- `gpon_measurements`: hypertable con contadores de tráfico por puerto GPON (MIB PON Huawei) y los conteos active/inactive/error del mismo escaneo

Compresión automática activada (3 días para ONT, 7 para GPON). El bps se calcula en SQL con `LAG` sobre los contadores, por lo que las consultas por rango de fechas son rápidas incluso con millones de filas.

### Migraciones

`Migrate` se ejecuta en cada arranque del servidor y de cada `ont scan`, y es idempotente. Tres detalles importan en producción:

- **Un solo `Migrate` a la vez**: toma un bloqueo de sesión (`pg_advisory_lock`) porque el servidor y los `ont scan` del cron migran a la vez, y dos `CREATE INDEX IF NOT EXISTS` simultáneos chocan con `duplicate key value violates unique constraint "pg_class_relname_nsp_index"`. El segundo arranque espera al primero.
- **Backfill en una transacción**: las migraciones que reescriben histórico (por ejemplo el relleno de `count_*`) van en una única transacción; si falla, se revierte entero y el siguiente arranque lo reintenta. Mientras corre, el servidor no abre el puerto.
- **Límite de descompresión**: TimescaleDB limita por sentencia cuántas tuplas puede descomprimir un `UPDATE` (`timescaledb.max_tuples_decompressed_per_dml_transaction`, 100 000 por defecto) y devuelve `SQLSTATE 53400` si se supera. Las migraciones que tocan chunks comprimidos lo desactivan con `SET LOCAL ... = 0` dentro de su transacción.

Con tablas grandes (cientos de millones de filas) una migración así puede tardar decenas de minutos: conviene parar el cron de `ont scan` y hacerla en ventana de mantenimiento.

## Configuración

Variables de entorno:

| Variable           | Default                                                       | Descripción                                  |
| ------------------ | ------------------------------------------------------------- | -------------------------------------------- |
| `DATABASE_URL`     | `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable` | DSN de PostgreSQL (la BD se crea sola si no existe) |
| `GOONT_ADDR`       | `0.0.0.0:8080`                                                | Dirección de escucha del servidor HTTP       |
| `GOONT_SNMP_CONNS` | `10`                                                          | Conexiones SNMP simultáneas por OLT          |
| `GOONT_MAX_OLTS`   | `32`                                                          | OLTs escaneándose en paralelo                |
| `GOONT_SCAN_TIMEOUT` | `15`                                                        | Timeout SNMP (segundos) por consulta durante `ont scan`; los valores por OLT solo se usan si son menores |
| `GOONT_SCAN_RETRIES` | `2`                                                         | Reintentos SNMP por consulta durante `ont scan`; ídem |
| `GOONT_SCAN_DEADLINE` | `270`                                                      | Deadline del ciclo de escaneo (segundos). Al vencer se conserva lo ya guardado y el lock se libera para el próximo cron. Con el cron de 15 min usar `780` (13 min) |
| `GOONT_DB_MAX_CONNS` | `10`                                                        | Conexiones del pool del servidor API (la CLI usa 3) |
| `GOONT_CACHE_TTL`  | `60`                                                          | Segundos que se cachean las respuestas JSON (`0` desactiva) |
| `GOONT_CACHE_MB`   | `64`                                                          | Memoria máxima de la caché en MiB                  |

## Despliegue

El servidor de destino no necesita acceso a internet. La configuración va en un `.env` (copia de `.env.example`), que nunca se sube al repositorio ni se pasa por línea de comandos.

```bash
# 1. En el equipo de build: construye la imagen y exporta el tar
make save                                         # dist/goont-<tag>.tar.gz

# 2. Transfiere al servidor (tar + Makefile + .env.example)
scp dist/goont-<tag>.tar.gz Makefile .env.example usuario@servidor:/opt/goont/

# 3. En el servidor
cd /opt/goont
docker load -i goont-<tag>.tar.gz
cp .env.example .env && nano .env                 # credenciales reales
make run-server                                   # crea el contenedor con --env-file .env
```

Gestión en el servidor (requiere el Makefile y el `.env` copiados):

```bash
make logs            # logs del servidor
make restart         # recrea el contenedor (aplica cambios del .env)
make stop            # detiene y elimina el contenedor
make run-scan        # ejecuta 'ont scan' (contenedor efímero, usa el .env)
make run-cli CMD='olt list'
```

Para escanear periódicamente, entrada de cron en el servidor:

```
*/15 * * * * cd /opt/goont && make run-scan >> /var/log/goont-scan.log 2>&1
```

El cron intenta cada 15 minutos, pero `ont scan` toma un bloqueo en Postgres
(`TryScanLock`): si el ciclo anterior sigue corriendo, el nuevo se omite. El
escaneo guarda por puerto apenas termina, con un deadline de
`GOONT_SCAN_DEADLINE` (780 s en el ejemplo de 15 min; 270 s es el default del
código): los OLTs/puertos que no responden a tiempo quedan como hueco y el ciclo
siguiente los reintenta. La cadencia real de cada OLT es la del ciclo que
alcanzó a escanearlo, no los 15 minutos del cron.

### Operación en el servidor (docker exec)

La CLI vive dentro del mismo contenedor; no requiere entrar al contenedor, solo `docker exec`:

```bash
docker exec goont goont olt add --ip 192.168.1.1 --community public   # agrega y valida por SNMP
docker exec goont goont olt list                                      # OLTs registrados
docker exec goont goont ont scan                                      # un ciclo de escaneo (manual)
```

Orden recomendado: agregar OLT → `ont scan` manual de prueba → configurar el cron. Si un OLT falla durante el escaneo, el ciclo continúa con los demás y el error queda en `docker logs`.

### API HTTP

| Endpoint | Descripción |
| --- | --- |
| `GET /api/v1/olt` | OLTs registrados |
| `GET /api/v1/olt/{ip}` | Detalle de un OLT |
| `GET /api/v1/olt/{ip}/ports` | Puertos GPON del OLT con conteo de ONTs |
| `GET /api/v1/olt/{ip}/onts?q=&gpon=` | Inventario de ONTs, buscable por descripción (DNI/RIF) o serial |
| `GET /api/v1/traffic/{ip}` | Tráfico total del OLT (suma de sus puertos) |
| `GET /api/v1/traffic/{ip}/ports` | Serie de todos los puertos GPON en una sola respuesta |
| `GET /api/v1/traffic/{ip}/{gpon}` | Tráfico del puerto GPON + conteo de ONTs por estado |
| `GET /api/v1/traffic/{ip}/{gpon}/{ont}` | Tráfico y señal óptica de un ONT |
| `GET /api/v1/health` | Estado del servicio |

Los endpoints de tráfico exigen `initDate` y `endDate` (formato RFC3339) y devuelven intervalos con bytes y bps calculados en la base de datos: se necesita al menos **dos escaneos** para que haya tráfico consultable. Los intervalos con tráfico físicamente imposible (> 2.5 Gbps o wraps de contador) se omiten, el OLT solo suma intervalos válidos, los campos ópticos sin medición se sirven como `null` y las filas incluyen `count_unmeasured`. El contrato completo está en [`docs/API.md`](docs/API.md).

Para las gráficas por puerto, `GET /api/v1/traffic/{ip}/ports` devuelve todas las series GPON del OLT en **una sola respuesta** (los mismos campos que el endpoint individual), de modo que el Dashboard no tenga que hacer N peticiones.

```bash
curl 'https://<servidor>/api/v1/traffic/192.168.1.1?initDate=2026-09-04T00:00:00Z&endDate=2026-09-05T00:00:00Z'
```

Las respuestas JSON `GET` se cachean en memoria del proceso (`GOONT_CACHE_TTL`, por defecto 60 s; `/health` nunca se cachea). Un rango de fechas que ya terminó se guarda 1 hora, porque sus datos no cambian. Cada respuesta indica `X-Cache: HIT|MISS` (y `Age` en los aciertos) para diagnosticar; si se ven consultas repetidas como `MISS`, la caché no está entrando en juego.

Variables del `.env` (todas opcionales, hay defaults):

| Variable           | Default                                     | Descripción                                        |
| ------------------ | ------------------------------------------- | -------------------------------------------------- |
| `DATABASE_URL`     | `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable` | DSN de PostgreSQL (la BD se crea sola si no existe) |
| `GOONT_ADDR`       | `0.0.0.0:8080`                              | Dirección de escucha del servidor API              |
| `GOONT_SNMP_CONNS` | `10`                                        | Conexiones SNMP simultáneas por OLT                |
| `GOONT_MAX_OLTS`   | `32`                                        | OLTs escaneándose en paralelo                      |
| `GOONT_DB_MAX_CONNS` | `10`                                      | Conexiones del pool del servidor API (la CLI usa 3) |
| `GOONT_CACHE_TTL`  | `60`                                        | Segundos que se cachean las respuestas JSON (`0` desactiva) |
| `GOONT_CACHE_MB`   | `64`                                        | Memoria máxima de la caché en MiB                  |

## Dependencias

- [gosnmp/gosnmp](https://github.com/gosnmp/gosnmp): Cliente SNMP
- [urfave/cli](https://github.com/urfave/cli): Framework CLI
- [jackc/pgx](https://github.com/jackc/pgx): Driver PostgreSQL
- [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter): Tablas en terminal

## Contribuciones

Las contribuciones son bienvenidas. Por favor, abre un issue o pull request en GitHub.

## Licencia

MIT
