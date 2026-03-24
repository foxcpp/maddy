# AGENTS.md — Maddy Mail Server

## Architecture

Maddy is a composable all-in-one mail server (MTA/MX/IMAP) written in Go. The core abstraction is the **module system**: every functional component (auth, storage, checks, targets, endpoints) implements `module.Module` from `framework/module/module.go` and registers itself via `module.Register(name, factory)` in an `init()` function.

- **`framework/`** — Stable, reusable packages (config parsing, module interfaces, address handling, error types, logging). Interfaces live here to avoid circular imports.
- **`internal/`** — All module implementations. Subdirectories map to module roles: `endpoint/` (protocol listeners), `target/` (delivery destinations), `auth/`, `check/` (message inspectors), `modify/` (header modifiers), `storage/`, `table/` (string→string lookups).
- **`maddy.go`** — Side-effect imports that pull all `internal/` modules into the binary, plus the `Run`/`moduleConfigure`/`RegisterModules` startup sequence.
- **`cmd/maddy/main.go`** — Thin entrypoint; imports root package for module registration, then calls `maddycli.Run()`.

Modules are wired together at runtime via `maddy.conf` configuration. Top-level blocks are lazily initialized through `module.Registry`. The **message pipeline** (`internal/msgpipeline/`) routes messages from endpoints through checks, modifiers, and to delivery targets based on sender/recipient matching rules.

## Build & Test

```sh
# Build (produces ./build/maddy by default):
./build.sh build

# Build with specific tags (e.g. for Docker):
./build.sh --tags "docker" build

# Unit tests (standard Go):
go test ./...

# Integration tests
cd tests && ./run.sh
```

The build embeds version via `-ldflags -X github.com/foxcpp/maddy.Version=...`. A C compiler is needed for SQLite support (`mattn/go-sqlite3`).

## Adding a New Module

1. Create a package under the appropriate `internal/` subdirectory (e.g. `internal/check/mycheck/`).
2. Implement `module.Module` plus the relevant role interface (`module.Check`, `module.DeliveryTarget`, `module.PlainAuth`, `module.Table`, etc.) from `framework/module/`.
3. Register in `init()`: `module.Register("check.mycheck", NewMyCheck)`. Use naming convention: `check.`, `target.`, `auth.`, `table.`, `modify.` prefixes.
4. Add a blank import `_ "github.com/foxcpp/maddy/internal/check/mycheck"` in `maddy.go`.
5. For checks: use the skeleton at `internal/check/skeleton.go` or `check.RegisterStatelessCheck` (see `internal/check/dns/` for a stateless example).

## Error Handling

Use `framework/exterrors` — not bare `fmt.Errorf`. Errors crossing module boundaries must carry:
- SMTP status info via `exterrors.SMTPError{Code, EnhancedCode, Message, CheckName/TargetName}`
- Temporary flag via `exterrors.WithTemporary`
- Module name field

Keep SMTP error messages generic (no server config details). Use `exterrors.WithFields` for unexpected errors. See `HACKING.md` for full guidelines.

## Key Conventions

- **No shared state between messages** — check/modifier code runs in parallel across messages.
- **Panic recovery** — any goroutine you spawn must recover panics to avoid crashing the server.
- **Address normalization** — domain parts must be U-labels with NFC normalization and case-folding. Use `framework/address.CleanDomain`.
- **Configuration parsing** — modules receive config via `config.Map` in their `Configure` method. See `framework/config/` and existing modules for the pattern.
- **Logging** — use `framework/log.Logger`, not `log` stdlib. Per-delivery loggers via `target.DeliveryLogger(...)`.

