# Repository Guidelines

## Project Structure & Module Organization
- `server/`: Go API server. Entry point is `server/cmd/memorix-server`; core layers live under `server/internal/` (`handler`, `service`, `repository`, `domain`, `middleware`, `config`).
- `openclaw-plugin/` and `opencode-plugin/`: TypeScript agent plugins.
- `claude-plugin/`: Bash hook-based plugin plus reusable skills.
- `site/`: Astro marketing/documentation site.
- `e2e/`: End-to-end scripts (bash + Python) that run against a live server.
- `docs/`: design notes; start with `docs/DESIGN.md`.

## Build, Test, and Development Commands
- `make build`: build `server/bin/memorix-server`.
- `make run MNEMO_DSN="user:pass@tcp(host:4000)/memorix?parseTime=true"`: build and run server locally.
- `make vet`: run `go vet ./...` in `server/`.
- `make test`: run Go unit tests with race detector.
- `make test-integration`: run TiDB repository integration tests (`-tags=integration`).
- `cd openclaw-plugin && npm run typecheck`: TypeScript type checks for OpenClaw plugin.
- `cd opencode-plugin && npm run typecheck`: TypeScript type checks for OpenCode plugin.
- `cd site && npm run dev` (or `npm run build`): run/build Astro site.

## Coding Style & Naming Conventions
- Go: target Go 1.22, keep files `gofmt`-clean, prefer table-driven tests for service/repository behavior.
- Shell: use `set -euo pipefail`; follow patterns in `claude-plugin/hooks/common.sh`.
- TypeScript: strict type-checking via `tsc --noEmit`; keep plugin source in `src/` (OpenCode) and top-level `*.ts` (OpenClaw).
- Naming: use descriptive package/file names (`memory.go`, `tenant_test.go`), and keep layer boundaries explicit (handler -> service -> repository).

## Testing Guidelines
- Unit tests live next to code as `*_test.go`; run with `make test`.
- Integration tests are under `server/internal/repository/tidb/*_integration_test.go`; run with `make test-integration` and a configured test database.
- E2E checks in `e2e/` require a running server and `MNEMO_TEST_USER_TOKEN` (see `e2e/README.md`).

## Commit & Pull Request Guidelines
- Follow observed history style: Conventional Commit-like prefixes (`feat:`, `fix:`, `chore:`) with imperative summaries.
- Keep commits focused by subsystem (server, plugin, site) and include tests with behavior changes.
- PRs should include: purpose, key changes, verification steps/commands, linked issue (if any), and screenshots for `site/` UI changes.
