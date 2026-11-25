# AGENTS.md

Guidance for AI agents working on GoONT: a Go tool for managing/monitoring Huawei OLT/ONT devices via SNMP. Module name is `goont`, Go 1.25.6.

## Commands

```bash
go build -o goont ./cmd/cli           # CLI binary
go build -o goont-server ./cmd/server # HTTP API server
go test ./...                         # tests unitarios (los de storage se saltan sin DB)
go vet ./... && gofmt -l .            # Only static checks available
go mod tidy                           # Run before committing
```

Requires a PostgreSQL with TimescaleDB running (default `localhost:5432`, user/password `postgres`, db `goont`). Hay tests unitarios; los de `storage/` se saltan si no hay PostgreSQL. No hay CI ni linter configurado; verificación = build + vet + gofmt + tests. For integration testing, `docker run -d --name goont-dev-timescale -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=goont -p 5432:5432 timescale/timescaledb:latest-pg16` works.

`make help` lists automation targets: `make check` (fmt+vet+build), `make build-cli`/`build-server`, `make image` (Docker multi-stage, alpine, both binaries, non-root), `make save` (exports `dist/goont-<tag>.tar.gz` for offline servers; transfer + `docker load` is manual, the Makefile has no ssh/scp). `IMAGE` is deliberately fully qualified as `docker.io/library/goont`: local docker is actually podman, which would otherwise brand the build as `localhost/goont`, while docker on the server resolves `goont:<tag>` to `docker.io/library/goont`. Runtime targets (`run-server`, `run-scan`, `run-cli CMD='...'`, `stop`/`restart`/`logs`) exec in docker and take config from `--env-file .env` (gitignored; `.env.example` documents the vars) — env is container config applied at `docker run`, never baked into the image; never hardcode credentials in the Makefile. Tag comes from `git describe --always --dirty`. Local docker is actually podman: image HEALTHCHECK is stripped from OCI-format builds, so run targets pass `--health-cmd` at `docker run` instead.

## Configuration (env vars, no flags/config files)

- `DATABASE_URL` (default `postgres://postgres:postgres@localhost:5432/goont?sslmode=disable`) — the database itself is auto-created if missing (`storage.Connect` catches PG error `3D000` and runs `CREATE DATABASE` against the `postgres` DB).
- `GOONT_ADDR` (default `0.0.0.0:8080`), `GOONT_SNMP_CONNS` (default 10, per-OLT pool size), `GOONT_MAX_OLTS` (default 32, parallel OLTs per scan).
- `GOONT_DB_MAX_CONNS` (default 10, API pool; the CLI always uses 3), `GOONT_CACHE_TTL` (seconds, default 60, `0` disables) and `GOONT_CACHE_MB` (default 64) configure the in-memory JSON response cache.
- `GOONT_SCAN_TIMEOUT` (seconds, default 15), `GOONT_SCAN_RETRIES` (default 2) and `GOONT_SCAN_DEADLINE` (seconds, default 270; the `.env.example` 15-minute cron uses 780) bound `ont scan`: the per-OLT `snmp_timeout`/`snmp_retries` only apply when lower, and the deadline cuts the cycle so the lock is free for the next cron.
- Schema is migrated idempotently on every server start and every CLI action (`storage.Migrate`) — do not run migrations by hand. `Migrate` holds a session advisory lock (`migrateLockKey`): the server and cron `ont scan` containers migrate concurrently and two `CREATE INDEX IF NOT EXISTS` on the same name abort with SQLSTATE 23505. Migrations that rewrite history run in one transaction (server does not listen until it commits) and must call `liftDecompressionLimit`, or TimescaleDB aborts them with SQLSTATE 53400 on compressed chunks.

## Architecture

- `cmd/cli` + `commands/` — CLI built on urfave/cli v3: `olt list|add|remove`, `ont scan`. Every action creates its own pgx pool via `commands.newStore` and closes it on exit.
- `cmd/server` + `handlers/` + `middleware/` — HTTP API on `GOONT_ADDR`, stdlib `net/http` only. Go 1.22+ ServeMux patterns (`GET /api/v1/olt/{ip}`, `r.PathValue`), middleware chain Logging → RecoverPanic → CORS plus Cache (in-memory LRU, JSON GETs except `/health`) and Timeout (110 s). `http.Server.WriteTimeout` is 120 s because traffic queries can take tens of seconds; do not lower it without a shorter request timeout. Traffic endpoints take `initDate`/`endDate` query params in RFC3339 and return per-interval deltas/bps.
- `ont scan` takes a Postgres advisory lock (`storage.TryScanLock`) so overlapping cron cycles skip instead of duplicating samples; the CLI pool is 3 connections to keep the server pool free. Each port persists its ONTs and GPON sample as soon as it finishes: partial ONTs keep the OIDs that responded (missing metrics go NULL) and a failed traffic walk leaves a gap for that port instead of aborting the OLT. The cycle has a `GOONT_SCAN_DEADLINE` (780 s in the 15-minute deployment, 270 s default) so a slow OLT cannot hold the lock and starve the next cron.
- `snmp/` — gosnmp v2c client (port 161). `Snmp` holds a **pool of reused connections per OLT** (`NewSnmp(..., conns)`); `acquire/release` distributes queries over it — do NOT go back to connect-per-query. OID constants are private in `snmp/types.go`. Serial numbers are hex-encoded. `GponTrafficPort()` reads one GPON **port**'s traffic from the Huawei PON MIB (`hwGponOltEthernetStatisticReceivedBytes`/`SendBytes`), not by summing ONTs; a failed port is skipped.
- `storage/` — PostgreSQL/TimescaleDB via `pgx/v5`. Hypertables `ont_measurements` (per-ONT, 3-day compression) and `gpon_measurements` (per-GPON-port, 7-day compression). Bulk writes use `pgx.CopyFrom`; ONT identity upserts use an `unnest(...)` array insert with `ON CONFLICT`. `gpon_measurements` also stores `count_active`/`count_inactive`/`count_error` for the port, computed from the ONTs read in the same scan cycle, so traffic queries never aggregate `ont_measurements`. Deltas/bps are computed **in SQL** with `LAG` window functions (`storage/traffic.go`), not in Go. Counter resets (wrapped counters) are skipped, not extrapolated. Per-ONT intervals are additionally capped by the contracted rate parsed from `line_profile` (`traffic.LineRateBps`, which normalizes human prefixes like `1000M`/`1000G` → `1.5x` burst margin → `traffic.OntMaxBps`, via `storage.ontMaxBps`), so a 60 Mbps plan never serves 300 Mbps artifact spikes; unparseable profiles fall back to the 2.5 Gbps port limit (the plan rate is a safety heuristic, not authoritative).
- `models/` — shared struct types. `models.Ont` (raw sample) uses `*int32` for SNMP values that may be missing (nil → NULL in PG); response models (`OntMeasurement`, `GponMeasurement`, `OltMeasurement`) keep plain types where missing renders as 0.

## Conventions

- Code uses `WaitGroup.Go` (Go 1.25 method) for goroutines (`commands/ont.go`) — do not rewrite it as `wg.Add`/`Done`.
- User-facing and CLI error strings are in Spanish ("Error al intentar..."); `storage/` and `handlers/` internal errors are in English, wrapped with `%w`. Match the style of the file you are editing.
- Concurrency: `ont scan` processes up to `GOONT_MAX_OLTS` OLTs in parallel and, inside each OLT, up to `GOONT_SNMP_CONNS` GPON ports (the same size as the SNMP pool); the OLT order is shuffled each cycle so slow OLTs don't always fall outside the deadline. `scanPort` persists per port: ONTs first (partial, missing counters NULL) and then the GPON sample, so a failure leaves a gap instead of aborting the OLT. One `now` timestamp per OLT scan keeps ONT and GPON rows aligned.
- JSON API: `GET /api/v1/traffic/{ip}` → OLT totals (sum of port counters), `/{ip}/ports` → all GPON port series in one response, `/{ip}/{gpon}` → port counter traffic + ONT status counts, `/{ip}/{gpon}/{ont}` → per-ONT detail. Traffic comes from port-level Huawei PON counters (`GponTrafficPort`); per-ONT counters are only for per-ONT endpoints. Inventory endpoints `GET /api/v1/olt/{ip}/ports` and `/api/v1/olt/{ip}/onts?q=<dni|serial>&gpon=<idx>` enumerate from the `onts` table (`desp` is the user's unique ID; search is `ILIKE` on description + serial, LIMIT 500).
