# AGENTS.md — Maddy Mail Server

## Architecture

Maddy is a composable all-in-one mail server (MTA/MX/IMAP) written in Go. The core abstraction is the **module system**: every functional component (auth, storage, checks, targets, endpoints) implements `module.Module` from `framework/module/module.go` and registers itself via `module.Register(name, factory)` in an `init()` function.

- **`framework/`** — Stable, reusable packages (config parsing, module interfaces, address handling, error types, logging). Interfaces live here to avoid circular imports.
- **`internal/`** — All module implementations. Subdirectories map to module roles: `endpoint/` (protocol listeners), `target/` (delivery destinations), `auth/`, `check/` (message inspectors), `modify/` (header modifiers), `storage/`, `table/` (string→string lookups).
- **`maddy.go`** — Side-effect imports that pull all `internal/` modules into the binary, plus the `Run`/`moduleConfigure`/`RegisterModules` startup sequence.
- **`cmd/maddy/main.go`** — Thin entrypoint; imports root package for module registration, then calls `maddycli.Run()`.

Modules are wired together at runtime via `maddy.conf` configuration. Top-level blocks are lazily initialized through `module.Registry`. The **message pipeline** (`internal/msgpipeline/`) routes messages from endpoints through checks, modifiers, and to delivery targets based on sender/recipient matching rules.

## RFC cross-verification

- Changes that affect standard protocol inputs or outputs, change observable semantics of protocol commands or interpretation of supplied values must be cross-verified for correctness against represtive RFC specifications. If unsure - ask the user for RFC reference. Use either tool available to fetch RFC text (RFC MCP, or Web Fetch from http://www.rfc-editor.org/info/rfcNNNN).
- maddy has the list of implemented specifications in docs/internals/specifications.md. Keep the list updated if implementing new specifications.

## Build & Test

```sh
# Build (produces ./build/maddy by default):
./build.sh build

# Build with specific tags (e.g. for Docker):
./build.sh --tags "docker" build

# Linters.
golangci-lint run

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

## Running server for manual testing

If you ever need to run the server for testing, do this:

1. Build server executable: go build ./cmd/maddy (no need to use build.sh for test builds).
2. Populate configuration.
    1. Determine test state directory to use, if test configuration needs persistent state (e.g. IMAP storage) - create a temporary subdirectory in /tmp, otherwise just use /tmp as is.
    2. Prefer to use minimal focused config file instead of entire default config to reduce noise unless task explicitly relates to default config.
    3. Put test state directory as both `state_dir` and `runtime_dir` in config.
    4. Use > 1024 ports for test endpoints as they do not require
    5. Be mindful of required directives - you need `hostname` and `tls` for smtp, other modules may need other directives.
4. Run the server as: `./maddy --config path-to-test-config run` (runs in foreground).

Example of test config:
```
state_dir /tmp
runtime_dir /tmp
hostname maddy.test

table.chain local_rewrites {
        optional_step file aliases
}

smtp tcp://127.0.0.1:5555 {
        modify {
                replace_rcpt &local_rewrites
        }

        tls off

        deliver_to lmtp tcp://127.0.0.1:6666
}
```
