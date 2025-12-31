# Repository Guidelines

## Project Structure & Module Organization
- `cmd/justdoit/`: CLI entrypoint (`main.go`).
- `internal/`: core packages (auth, config, Google API wrappers, CLI commands, agenda logic).
- `internal/cli/`: Cobra commands (`add`, `done`, `view`, `setup`, `config`).
- `internal/google/`: Google Tasks + Calendar clients.
- `internal/sync/`: task+calendar orchestration (time‑blocking).
- `internal/metadata/`: helpers for storing IDs/flags in notes.
- Root files: `README.md`, `Makefile`, `go.mod`/`go.sum`.

## Build, Test, and Development Commands
- `make build`: builds local binary `./justdoit`.
- `make install`: installs to `/opt/homebrew/bin/justdoit`.
- `make setup`: runs interactive setup.
- `make reset`: removes local config/token for a fresh setup.
- `make tidy`: runs `go mod tidy`.
- Direct build: `go build -o /opt/homebrew/bin/justdoit ./cmd/justdoit`.

## Coding Style & Naming Conventions
- Use `gofmt` for all Go files (`gofmt -w cmd internal`).
- Keep packages small and purpose‑driven under `internal/`.
- Prefer explicit names (e.g., `CreateTaskWithParent`, `ListCalendars`).

## Testing Guidelines
- No test framework is set up yet.
- If you add tests, follow Go conventions:
  - Files: `*_test.go`
  - Run: `go test ./...`
  - Keep tests close to the package they cover.

## Commit & Pull Request Guidelines
- Commit messages are short, imperative, and scoped (e.g., “Add Makefile”).
- Include a clear summary and mention CLI behavior changes in the PR body.
- For UX changes, attach a terminal screenshot or a short before/after note.

## Security & Configuration
- Do **not** commit secrets or OAuth credentials.
- Local config/token live in `~/.config/justdoit/`.
- OAuth credentials should be stored at:
  - `~/.config/justdoit/credentials.json` (macOS also supports `~/Library/Application Support/justdoit/`).
