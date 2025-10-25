<!-- Auto-generated guidance for AI coding agents. Keep short and specific to this repo. -->
# DBX — AI Coding Agent Instructions

This file contains concise, actionable guidance to help AI coding agents work productively in the `gostratum/dbx` repository. Keep changes minimal and only edit when repository structure or conventions change.

Summary
- DBX is a thin, Fx-first, GORM-backed database module for the Gostratum ecosystem. Key responsibilities: provide configured *gorm.DB* instances, run SQL/embed migrations, register health checks, and expose transaction helpers.

Where to look first
- `README.md` — high-level overview and usage examples (start here).
- `config.go` — authoritative defaults, validation, and config prefix `db` used by `core/configx`.
- `migrate/` — migration helpers and config; `migrate/migrate.go` is the public API for CLI/consumers.
- `tx.go` — transaction helpers and `TxManager`/`TxWrapper` patterns.
- `examples/simple-dbx/` — runnable example showing module wiring (`core.New`, `dbx.Module`, `fx.Invoke`).

Important patterns & conventions (do not invent alternatives)
- Fx-first composition: no package-level globals. Modules are provided via `dbx.Module(...)` and composed into applications with `core.New(...)` + `fx` invocations (see `examples/simple-dbx/main.go`).
- Config via `core/configx` with prefix `db`. The module expects configs under `db.databases.<name>`. Environment variables use the `STRATUM_` prefix when loaded via core/configx.
- Migrations:
  - Two sources supported: filesystem (`file://./migrations`) and embedded (`embed://`). The migrate package exposes `Up`, `Down`, `Steps`, `To`, `Force`, and `Drop` helpers.
  - `DatabaseConfig.MigrationSource` may be `"embed://"` or `file://<path>`; `AutoMigrate` is intentionally disabled by default and guarded in validation.
- Transactions: preferred helpers are `dbx.WithTx`, `dbx.WithTxContext`, or use `TxManager`/`TxWrapper` when savepoints or explicit control are needed.

Developer workflows & commands
- Build/tests (standard Go): `go test ./...` and `go vet ./...`. The repo uses modules (`go.mod`) — run with `GOFLAGS` or `CGO_ENABLED=0` if cross-building.
- Run example application: open `examples/simple-dbx` and run `go run ./` (it uses `core.New` and will attempt DB connections; set `STRATUM_DB_DATABASES_PRIMARY_DSN` or supply a local Postgres).
- Run migrations programmatically: use `migrate.UpFromDatabaseConfig(ctx, dbConfig)` or CLI-like helpers in `migrate` (pass DSN + options). For embedded migrations the repo exposes an `EmbeddedMigrations` FS under `migrate/files`.

Testing notes & safety
- Migration and auto-migrate are guarded: `AutoMigrate` defaults to false; do not enable in production. Tests that touch DB should set up a disposable Postgres instance or use an in-memory test DB and set `AutoMigrate=true` only for test scope.
- Config validation is strict — `Config.Validate()` will fail if `default` DB is missing or no databases configured. Use `DefaultConfig()` in tests to get sane defaults.

Common code idioms to follow
- Prefer explicit DI via Fx. New code should accept `*gorm.DB`, `*dbx.Provider`, or `*dbx.TxManager` via function parameters and not via globals.
- Use context-aware DB calls: `db.WithContext(ctx)....` so tracing/logging/timeout propagate.
- Logging and errors: follow the project’s pattern of wrapping errors with context (see `migrate` error wrappers) and use `core/logx`/Zap structured fields in examples.

Files to reference when making changes
- `README.md`, `config.go`, `migrate/`, `tx.go`, `examples/simple-dbx/main.go` — copy snippets from these when showing examples.

When to ask the human
- Config defaults: if you need to change defaults that could affect production (connection pools, AutoMigrate), ask a maintainer.
- New external dependencies: propose them and justify size/maintenance impact.

If you edit code
- Run `go test ./...` locally and ensure no new lint/type errors. Prefer adding a small unit test for new behavior (happy path + 1 failure case).
- Keep public APIs stable (Module options, Provider methods, TxManager) unless asked to make breaking changes — document any breaking changes in `CHANGELOG.md`.

Questions for the maintainer
- Do you want examples that demonstrate migrations with embedded SQL files included in the repo? If yes, I can add a small migration set under `migrate/files` and an example invocation.

----
Update this file only when repo structure or conventions change. Keep it concise and example-led.
