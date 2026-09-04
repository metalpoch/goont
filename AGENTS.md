# AGENTS.md

Guidance for AI agents working on GoONT: a Go tool for managing/monitoring Huawei OLT/ONT devices via SNMP. Module name is `goont`, Go 1.25.6.

## Commands

```bash
go build -o goont ./cmd/cli           # CLI binary
go build -o goont-server ./cmd/server # HTTP API server
go test ./...                         # Standard tests (none exist yet)
go vet ./... && gofmt -l .            # Only static checks available
go mod tidy                           # Run before committing
```

Requires a PostgreSQL with TimescaleDB running (default `localhost:5432`, user/password `postgres`, db `goont`). No tests, no CI, no linter config exist; verification means build + vet + gofmt. For integration testing, `docker run -d --name goont-dev-timescale -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=goont -p 5432:5432 timescale/timescaledb:latest-pg16` works.

`make help` lists automation targets: `make check` (fmt+vet+build), `make build-cli`/`build-server`, `make image` (Docker multi-stage, alpine, both binaries, non-root), `make save` (exports `dist/goont-<tag>.tar.gz` for offline servers; transfer + `docker load` is manual, the Makefile has no ssh/scp). `IMAGE` is deliberately fully qualified as `docker.io/library/goont`: local docker is actually podman, which would otherwise brand the build as `localhost/goont`, while docker on the server resolves `goont:<tag>` to `docker.io/library/goont`. Runtime targets (`run-server`, `run-scan`, `run-cli CMD='...'`, `stop`/`restart`/`logs`) exec in docker and take config from `--env-file .env` (gitignored; `.env.example` documents the vars) — env is container config applied at `docker run`, never baked into the image; never hardcode credentials in the Makefile. Tag comes from `git describe --always --dirty`. Local docker is actually podman: image HEALTHCHECK is stripped from OCI-format builds, so run targets pass `--health-cmd` at `docker run` instead.

## Configuration (env vars, no flags/config files)

- `DATABASE_URL` (default `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable`) — the database itself is auto-created if missing (`storage.Connect` catches PG error `3D000` and runs `CREATE DATABASE` against the `postgres` DB).
- `GOONT_ADDR` (default `0.0.0.0:8080`), `GOONT_SNMP_CONNS` (default 10, per-OLT pool size), `GOONT_MAX_OLTS` (default 32, parallel OLTs per scan).
- Schema is migrated idempotently on every server start and every CLI action (`storage.Migrate`) — do not run migrations by hand.

## Architecture

- `cmd/cli` + `commands/` — CLI built on urfave/cli v3: `olt list|add|remove`, `ont scan`. Every action creates its own pgx pool via `commands.newStore` and closes it on exit.
- `cmd/server` + `handlers/` + `middleware/` — HTTP API on `GOONT_ADDR`, stdlib `net/http` only. Go 1.22+ ServeMux patterns (`GET /api/v1/olt/{ip}`, `r.PathValue`), middleware chain Logging → RecoverPanic → CORS. Traffic endpoints take `initDate`/`endDate` query params in RFC3339 and return per-interval deltas/bps.
- `snmp/` — gosnmp v2c client (port 161). `Snmp` holds a **pool of reused connections per OLT** (`NewSnmp(..., conns)`); `acquire/release` distributes queries over it — do NOT go back to connect-per-query. OID constants are private in `snmp/types.go`. Serial numbers are hex-encoded. `GponTraffic()` reads GPON **port** traffic from standard IF-MIB `ifHCInOctets`/`ifHCOutOctets` (not from summing ONTs).
- `storage/` — PostgreSQL/TimescaleDB via `pgx/v5`. Hypertables `ont_measurements` (per-ONT, 3-day compression) and `gpon_measurements` (per-GPON-port, 7-day compression). Bulk writes use `pgx.CopyFrom`; ONT identity upserts use an `unnest(...)` array insert with `ON CONFLICT`. Deltas/bps are computed **in SQL** with `LAG` window functions (`storage/traffic.go`), not in Go. Counter resets (wrapped counters) are skipped, not extrapolated.
- `models/` — shared struct types. `models.Ont` (raw sample) uses `*int32` for SNMP values that may be missing (nil → NULL in PG); response models (`OntMeasurement`, `GponMeasurement`, `OltMeasurement`) keep plain types where missing renders as 0.

## Conventions

- Code uses `WaitGroup.Go` (Go 1.25 method) for goroutines (`commands/utils.go`, `commands/ont.go`) — do not rewrite it as `wg.Add`/`Done`.
- User-facing and CLI error strings are in Spanish ("Error al intentar..."); `storage/` and `handlers/` internal errors are in English, wrapped with `%w`. Match the style of the file you are editing.
- Concurrency: `ontScanner` scans GPON ports with a semaphore sized to the SNMP pool (`snmp.DefaultConns`); `ont scan` processes up to `GOONT_MAX_OLTS` OLTs in parallel, one Snmp pool + one `now` timestamp per OLT scan cycle (ONT rows and GPON rows share the same timestamp, which the traffic queries rely on when joining counts).
- JSON API: `GET /api/v1/traffic/{ip}` → OLT totals (sum of port counters), `/{ip}/{gpon}` → port counter traffic + ONT status counts, `/{ip}/{gpon}/{ont}` → per-ONT detail. Traffic comes from port-level IF-MIB counters; per-ONT counters are only for per-ONT endpoints.
