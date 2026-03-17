# Agent Guidelines for GoONT

Guidelines for AI agents working on the GoONT codebase. Covers build commands, testing, linting, and code style.

## Build Commands

### Building
```bash
go build -o goont ./cmd/cli          # Build binary
go install ./cmd/cli                 # Install to $GOPATH/bin
GOOS=linux GOARCH=amd64 go build -o goont-linux-amd64 ./cmd/cli  # Cross‑compile
go build -race -o goont ./cmd/cli    # With race detection
```

### Dependencies
```bash
go mod download     # Download dependencies
go mod tidy         # Tidy module files (run before commits)
go mod verify       # Verify dependencies
```

## Testing

### Running Tests
```bash
go test ./...                         # All tests
go test -v ./...                      # Verbose output
go test -race ./...                   # Race detection
go test -cover ./...                  # Coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go test -v ./commands -run TestOltAdd # Specific test
go test -bench=. ./...                # Benchmarks
```

### Test Structure
- Test files: `*_test.go` in same package.
- Use `testing` package.
- No existing tests; follow standard Go patterns.

## Linting and Code Quality

### Formatting
```bash
go fmt ./...          # Format all Go files
gofmt -d .            # Check formatting
```

### Static Analysis
```bash
go vet ./...                          # Vet for suspicious constructs
golangci-lint run ./...               # If golangci-lint installed
```

### Suggested Linter Configuration
If adding `.golangci.yml`, enable: `gofmt`, `govet`, `staticcheck`, `gosimple`, `unused`.

## Code Style Guidelines

### General Principles
- Follow [Effective Go](https://go.dev/doc/effective_go).
- Clear, maintainable code with meaningful names.
- Small, single‑responsibility functions.
- Prefer simplicity.

### File Organization
- One package per file.
- Lowercase filenames, underscores only when needed.
- `main` in `cmd/cli/main.go`.
- Group related types/functions in same package.

### Package Structure
- `cmd/cli` – CLI entry point.
- `commands` – CLI command implementations.
- `snmp` – SNMP client logic.
- `storage` – Database operations and types.

### Import Ordering
Group imports, separated by blank line:
1. Standard library
2. Internal packages (`goont/`)
3. Third‑party packages

Example:
```go
import (
    "context"
    "fmt"

    "goont/snmp"

    "github.com/urfave/cli/v3"
)
```

### Naming Conventions
- `camelCase` for locals and private functions.
- `PascalCase` for exported identifiers.
- Acronyms uppercase (`SNMP`, `OLT`, `ONT`).
- Interface names end with `‑er` when appropriate (`Scanner`).
- Database functions clear (`InsertOLT`, `GetOLTByID`).

### Error Handling
- Always handle errors; never ignore.
- Wrap errors with `fmt.Errorf` and `%w`.
- Error messages lower‑case, concise (Spanish or English, consistent).
- Return `nil` on success.
- Use `defer` for cleanup (close connections, files).

Example:
```go
func doSomething() error {
    client, err := storage.NewOltDB(path)
    if err != nil { return fmt.Errorf("create client: %w", err) }
    defer client.Close()
    // ...
}
```

### Logging
- Use `log` package for errors/important events.
- Avoid logging sensitive info (IPs, credentials).
- CLI: user‑friendly messages via `fmt.Printf`, errors via `log.Fatal` or returning error.

### Types and Structs
- Define structs in appropriate package (`storage.OLT`, `snmp.Ont`).
- Use `time.Time` for timestamps.
- Use `int32`/`int64`/`uint64` as appropriate for SNMP values.
- Document exported types with a comment.

### Concurrency
- Use `sync.WaitGroup` and channels for parallelism.
- Limit concurrency with semaphore channel (see `commands/utils.go:ontScanner`).
- Protect shared data with mutexes if needed.

### SQL and Database
- Parameterized queries (`?` placeholders).
- Always check `rows.Err()` after iterating rows.
- Wrap database errors with context (`%w`).
- Use `sql.Null*` for nullable columns if needed.

### CLI Commands
- Commands are `*cli.Command` variables in `commands` package.
- `Usage` text in Spanish (existing code).
- Flags with clear names and defaults.
- Action functions return error; CLI framework handles logging.

## Project‑Specific Conventions

### SNMP Constants
- OID constants in `snmp/types.go` (private).
- Hexadecimal encoding for serial numbers (`snmp/snmp.go:143`).

### Language
- User‑facing strings in Spanish (e.g., “Error al intentar…”).
- Internal errors/logs can be English.
- Keep consistency with existing code.

### Database Files
- Main OLT database: `olt.db` next to executable.
- ONT measurement databases separate per OLT IP.
- Do not commit database files to version control.

## Commit Guidelines
- Clear commit messages in imperative mood.
- Short summary (≤50 chars), blank line, optional description.
- Reference issues/PRs when applicable.
- Run `go mod tidy` before committing.

## Additional Notes
- No Cursor/Copilot rules defined.
- Go 1.25.6 (`go.mod`).
- Dependencies managed via Go modules.
- No CI/CD pipelines yet; consider adding GitHub Actions.

---

*Last updated: 2026-03-13*
