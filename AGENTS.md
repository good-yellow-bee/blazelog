# Repository Guidelines

## Project Structure & Module Organization
- `cmd/` holds entrypoints (`cmd/agent`, `cmd/server`, `cmd/blazectl`).
- `internal/` contains private packages (API, alerting, parser, storage, web UI, etc.).
- `pkg/` exposes public packages such as `pkg/config`.
- `internal/web/templates/` stores `*.templ` UI templates; built CSS lives in `web/static/css/`.
- `proto/` defines gRPC schemas; generated code lands in `internal/proto/`.
- `configs/`, `deployments/`, and `docs/` store examples, ops assets, and documentation.
- `e2e/` contains Playwright tests and fixtures.

## Build, Test, and Development Commands
- `make build` builds all binaries into `build/` (`blazelog`, `blazelog-agent`, `blazelog-server`).
- `make test` runs `go test -race` with coverage output at `coverage.out`.
- `make lint` runs `go vet` and `golangci-lint` if installed.
- `make fmt` applies `gofmt` across the repo.
- `make templ-generate` and `make web-build` regenerate Templ and Tailwind artifacts.
- `make dev-web` runs Templ + Tailwind watchers for UI work.
- Local run: `./build/blazelog-server --config configs/server.yaml`.
- E2E: `cd e2e && npm install && npm test`.

## Coding Style & Naming Conventions
- Go code is formatted with `gofmt` (tabs, standard Go layout) and linted via `golangci-lint`.
- Prefer clear error wrapping (`fmt.Errorf("context: %w", err)`).
- Tests use `*_test.go` and table-driven `TestXxx` patterns.
- Templ sources are `*.templ`; generated files end with `*_templ.go` (do not edit).

## Testing Guidelines
- Unit tests: `make test` or `go test ./internal/...`.
- Integration tests (require ClickHouse): `go test -tags=integration ./...`.
- Coverage HTML: `make test-coverage` generates `coverage.html`.

## Commit & Pull Request Guidelines
- Recent history uses conventional prefixes like `feat:`, `fix:`, `chore:`, `docs:`; keep summaries short and imperative.
- PRs should be focused, include tests, and update docs/configs when behavior changes.
- For UI changes, include screenshots or GIFs; link related issues when applicable.

## Security & Configuration Tips
- Keep secrets in environment variables (`BLAZELOG_JWT_SECRET`, `BLAZELOG_CSRF_SECRET`, `BLAZELOG_MASTER_KEY`).
- Use `configs/` and `docs/CONFIGURATION.md` for reference; never commit real credentials.
