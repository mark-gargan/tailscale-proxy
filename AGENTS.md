# Repository Guidelines

## Project Structure & Module Organization
- `cmd/tailscale-proxy/`: Service entrypoint (`main.go`).
- `internal/`: App packages (business logic, adapters).
- `pkg/`: Reusable libraries (no app-specific deps).
- `configs/`: Config samples (e.g., `config.yaml`, `.env.sample`).
- `scripts/`: Dev/CI helper scripts.
- `test/`: Integration/e2e tests and fixtures.
- `.github/workflows/`: CI pipelines.
If a folder is missing, create it when adding related code.

## Build, Test, and Development Commands
- `go build ./cmd/tailscale-proxy`: Build the binary.
- `go run ./cmd/tailscale-proxy`: Run locally (reads env/configs).
- `go test ./...`: Run unit tests.
- `go test -race -cover ./...`: Run with race detector and coverage.
- `go vet ./...`: Static checks.
- `golangci-lint run`: Lint (if configured).
- `make build | make test | make lint`: Preferred wrappers when a `Makefile` is present.

## Coding Style & Naming Conventions
- Formatting: `gofmt -s` and `goimports` (CI should reject unformatted code).
- Packages: short, lower-case (`internal/proxy`, `internal/config`).
- Files: lower_snake_case (`http_proxy.go`).
- Identifiers: `PascalCase` for exported, `camelCase` for internal; errors end with `Err` for vars (e.g., `var ErrAuthFailed`).
- Keep functions focused; prefer small, composable units; avoid stutter (`proxy.Proxy` → `proxy.Service`).

## Testing Guidelines
- Framework: Go `testing`; table-driven tests recommended.
- Layout: Unit tests near code (`*_test.go`), integration in `test/`.
- Names: `TestXxx`, `BenchmarkXxx`; use subtests for variants.
- Run: `go test -race -cover ./...`; aim for ≥80% coverage on changed packages.
- Mocks: Prefer interfaces + small fakes over heavy mocking.

## Commit & Pull Request Guidelines
- Commits: Conventional Commits (e.g., `feat(proxy): add ACL check`).
- PRs: concise description, rationale, and testing notes; link issues; include config/env notes and example commands.
- Size: keep PRs small and focused; add screenshots/log excerpts when behavior changes.

## Security & Configuration Tips
- Secrets: never commit keys; load via env (e.g., `TS_AUTHKEY`) or `.env.local` (git-ignored). Provide `.env.sample`.
- Config: prefer explicit flags/env (`LISTEN_ADDR`, `TS_SOCKET`) over implicit defaults.
- Local dev: use ephemeral keys/accounts; avoid binding to `0.0.0.0` unless required.
