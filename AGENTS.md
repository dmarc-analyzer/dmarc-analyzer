# Repository Guidelines

## Project Structure & Module Organization
- `backend/` contains the Go API, report parsing logic, DB models, and AWS S3/SQS integrations.
- `backend/cmd/` holds entry points (`server`, `consumer`, `backfill`, `generate_sql`).
- `frontend/` is a Vue 3 + Vite UI (views in `frontend/src/views`, shared components in `frontend/src/components`).
- `backend/schema.sql` is the PostgreSQL schema used for local/dev setup.
- `bin/` stores built binaries from the Makefile (`consumer`, `backfill`).

## Build, Test, and Development Commands
- `go run ./backend/cmd/server/server.go` starts the API server (default port 6767).
- `make build-consumer` / `make run-consumer` builds or runs the SQS consumer binary.
- `make build-backfill` / `make run-backfill` builds or runs the S3 backfill tool.
- `go run ./backend/cmd/backfill/backfill.go` runs backfill without Make.
- `cd frontend && yarn dev` starts the Vite dev server.
- `cd frontend && yarn build` builds the frontend for production.
- `docker-compose up -d` uses `docker-compose.yml` to run the app + Postgres.

## Coding Style & Naming Conventions
- Go code follows standard `gofmt` formatting; keep package names lowercase and exported identifiers in `CamelCase`.
- Vue components use `PascalCase` filenames (e.g., `DetailDialog.vue`). Keep utilities and services in `frontend/src/utils` and `frontend/src/services`.
- Frontend linting uses `eslint` via `cd frontend && yarn lint` (config in `frontend/eslint.config.mjs`).

## Testing Guidelines
- Go unit tests live alongside code in `*_test.go` (e.g., `backend/util/domain_test.go`).
- Run all Go tests with `go test ./...` from the repo root.
- If the local database doesn't exist yet, create it first: `createdb dmarc_analyzer`.
- Use the local DB URL when running tests: `DATABASE_URL=postgres://localhost:5432/dmarc_analyzer?sslmode=disable go test ./...`.
- The frontend currently has no test script in `frontend/package.json`; add one if introducing UI tests.

## Commit & Pull Request Guidelines
- Commit messages must be in English, short, and imperative; some use conventional prefixes like `fix:`. Follow this style for consistency.
- Git commits must be created with `-s -S` (example: `git commit -s -S -m "fix: ..."`) to sign off and GPG-sign changes.
- PRs should include a clear description, links to related issues, and any config or schema changes.
- If UI changes are included, add before/after screenshots for `frontend/` updates.

## Configuration & Security Notes
- Use a root `.env` file for AWS and DB credentials (see `README.md` for required variables).
- Keep secrets out of the repo; prefer local env files and CI secrets for deployments.
