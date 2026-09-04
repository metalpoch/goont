# GoONT - Herramienta de Gestión de OLT/ONT

GoONT es una herramienta de línea de comandos (CLI) escrita en Go para la gestión y monitoreo de equipos OLT (Optical Line Terminal) y ONT (Optical Network Terminal) en redes GPON mediante protocolo SNMP.

## Características

- **Gestión de OLTs**: Agregar, listar y eliminar OLTs de la base de datos
- **Escaneo de ONTs**: Obtención automática de información de ONTs conectados a cada OLT
- **Tráfico de puertos GPON**: Contadores oficiales del puerto vía IF-MIB (`ifHCInOctets`/`ifHCOutOctets`), sin sumar ONTs
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
├── snmp/                # Cliente SNMP (pool de conexiones, OIDs Huawei GPON + IF-MIB)
└── storage/             # PostgreSQL/TimescaleDB (migraciones, escritura masiva, consultas)
```

## Base de Datos

PostgreSQL + TimescaleDB. Las tablas se crean automáticamente al primer arranque:

- `olts`: registro de OLTs (IP, community, timeouts)
- `onts`: identidad de cada ONT visto (IP, puerto GPON, índice, serial)
- `ont_measurements`: hypertable con mediciones por ONT (contadores, señal, estado)
- `gpon_measurements`: hypertable con contadores de tráfico por puerto GPON (IF-MIB)

Compresión automática activada (3 días para ONT, 7 para GPON). El bps se calcula en SQL con `LAG` sobre los contadores, por lo que las consultas por rango de fechas son rápidas incluso con millones de filas.

## Configuración

Variables de entorno:

| Variable           | Default                                                       | Descripción                                  |
| ------------------ | ------------------------------------------------------------- | -------------------------------------------- |
| `DATABASE_URL`     | `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable` | DSN de PostgreSQL (la BD se crea sola si no existe) |
| `GOONT_ADDR`       | `0.0.0.0:8080`                                                | Dirección de escucha del servidor HTTP       |
| `GOONT_SNMP_CONNS` | `10`                                                          | Conexiones SNMP simultáneas por OLT          |
| `GOONT_MAX_OLTS`   | `32`                                                          | OLTs escaneándose en paralelo                |

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
0 */5 * * * cd /opt/goont && make run-scan >> /var/log/goont-scan.log 2>&1
```

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
| `GET /api/v1/traffic/{ip}` | Tráfico total del OLT (suma de sus puertos) |
| `GET /api/v1/traffic/{ip}/{gpon}` | Tráfico del puerto GPON + conteo de ONTs por estado |
| `GET /api/v1/traffic/{ip}/{gpon}/{ont}` | Tráfico y señal óptica de un ONT |
| `GET /api/v1/health` | Estado del servicio |

Los endpoints de tráfico exigen `initDate` y `endDate` (formato RFC3339) y devuelven intervalos con bytes y bps calculados en la base de datos: se necesita al menos **dos escaneos** para que haya tráfico consultable.

```bash
curl 'https://<servidor>/api/v1/traffic/192.168.1.1?initDate=2026-09-04T00:00:00Z&endDate=2026-09-05T00:00:00Z'
```

Variables del `.env` (todas opcionales, hay defaults):

| Variable           | Default                                     | Descripción                                        |
| ------------------ | ------------------------------------------- | -------------------------------------------------- |
| `DATABASE_URL`     | `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable` | DSN de PostgreSQL (la BD se crea sola si no existe) |
| `GOONT_ADDR`       | `0.0.0.0:8080`                              | Dirección de escucha del servidor API              |
| `GOONT_SNMP_CONNS` | `10`                                        | Conexiones SNMP simultáneas por OLT                |
| `GOONT_MAX_OLTS`   | `32`                                        | OLTs escaneándose en paralelo                      |

## Dependencias

- [gosnmp/gosnmp](https://github.com/gosnmp/gosnmp): Cliente SNMP
- [urfave/cli](https://github.com/urfave/cli): Framework CLI
- [jackc/pgx](https://github.com/jackc/pgx): Driver PostgreSQL
- [olekukonko/tablewriter](https://github.com/olekukonko/tablewriter): Tablas en terminal

## Contribuciones

Las contribuciones son bienvenidas. Por favor, abre un issue o pull request en GitHub.

## Licencia

MIT
