# Development Guide

A detailed walkthrough for setting up a local dev loop, running tests,
regenerating generated code, and getting changes merged.

For a one-paragraph quickstart, see [`CONTRIBUTING.md`](../CONTRIBUTING.md).

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Repository Layout Recap](#repository-layout-recap)
- [Local Setup](#local-setup)
- [Running the Stack Locally](#running-the-stack-locally)
- [Running Without AWS](#running-without-aws)
- [Tests](#tests)
- [Regenerating the Database Schema](#regenerating-the-database-schema)
- [Regenerating API Routes & Clients](#regenerating-api-routes--clients)
- [Frontend Workflow](#frontend-workflow)
- [Common Tasks](#common-tasks)
- [Debugging Tips](#debugging-tips)
- [CI Overview](#ci-overview)
- [Commit & PR Conventions](#commit--pr-conventions)

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.25+ | `go.mod` declares 1.25. |
| Node | 22+ | The Dockerfile uses Node 22; locally 20+ works for most things. |
| Yarn | 1.x | `package-lock.json` is committed so npm works too. Most scripts in AGENTS.md mention `yarn`; either is fine. |
| PostgreSQL | 14+ | Local install or a Docker container. |
| `openapi-generator` | v7.x | For regenerating routes & clients. `brew install openapi-generator` on macOS; the CI workflow uses a Docker shim. |
| `psql`, `createdb`, `dropdb`, `pg_dump` | — | For schema regen. |
| Docker | optional | For building the container image. |
| AWS account | optional | Only needed if you want to test live ingestion end-to-end. |

---

## Repository Layout Recap

See [`docs/ARCHITECTURE.md`](ARCHITECTURE.md#backend-packages) for the
full breakdown. At a glance:

```
backend/cmd/server      ← HTTP API + SPA
backend/cmd/consumer    ← SQS-driven ingester
backend/cmd/backfill    ← one-shot S3 importer
backend/cmd/generate_sql.go ← dev-only: dumps schema.sql
backend/handler         ← API handlers + generated routes
backend/model           ← DB + XML models
backend/messageprocessor ← SQS poll loop
backend/process.go      ← MIME / gzip / zip / XML decoding
api/openapi.json        ← API source of truth
frontend/               ← Vue 3 SPA
scripts/gen-routes.sh   ← regenerates Go route/handler bindings
```

---

## Local Setup

### 1. Clone

```sh
git clone https://github.com/dmarc-analyzer/dmarc-analyzer.git
cd dmarc-analyzer
```

### 2. Postgres

Pick one:

```sh
# A: Local Postgres (e.g. via brew, apt)
createdb dmarc_analyzer
psql -d dmarc_analyzer -f backend/schema.sql

# B: Docker
docker run -d --name dmarc-pg \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=dmarc_analyzer -p 5432:5432 \
  -v "$PWD/backend/schema.sql:/docker-entrypoint-initdb.d/schema.sql:ro" \
  postgres:14
```

Verify:

```sh
psql 'postgres://localhost:5432/dmarc_analyzer?sslmode=disable' \
     -c '\dt dmarc_report_entries'
```

### 3. Backend deps

```sh
go mod download
```

### 4. Frontend deps

```sh
cd frontend
yarn install
cd ..
```

---

## Running the Stack Locally

You'll typically run **server** and **frontend dev server** in parallel.
The **consumer** is only needed when you want to test live AWS ingestion;
otherwise insert test data manually (see [Running Without AWS](#running-without-aws)).

### Backend server

```sh
DATABASE_URL='postgres://localhost:5432/dmarc_analyzer?sslmode=disable' \
  go run ./backend/cmd/server/server.go
# server listens on http://127.0.0.1:6767
```

Sanity-check:

```sh
curl -s http://127.0.0.1:6767/api/domains
# expect: []  (empty array when DB is empty)
```

### Frontend dev server

```sh
cd frontend
yarn dev
# Vite serves on http://localhost:3000
# /api requests are proxied to http://127.0.0.1:6767
```

Open <http://localhost:3000>. Hot-reload works for `.vue` and `.ts` files.

### Consumer (optional, AWS required)

Only if you have a real S3 bucket + SQS queue:

```sh
DATABASE_URL='postgres://localhost:5432/dmarc_analyzer?sslmode=disable' \
  S3_BUCKET_NAME=your-org-dmarc-reports \
  SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/your-org-dmarc-reports \
  AWS_REGION=us-east-1 \
  AWS_ACCESS_KEY_ID=AKIA... \
  AWS_SECRET_ACCESS_KEY=... \
  go run ./backend/cmd/consumer/consumer.go
```

### Backfill (optional, AWS required)

```sh
DATABASE_URL='postgres://localhost:5432/dmarc_analyzer?sslmode=disable' \
  S3_BUCKET_NAME=your-org-dmarc-reports \
  AWS_REGION=us-east-1 \
  AWS_ACCESS_KEY_ID=AKIA... \
  AWS_SECRET_ACCESS_KEY=... \
  go run ./backend/cmd/backfill/backfill.go
```

---

## Running Without AWS

Most frontend and API work doesn't need a live AWS connection. Insert a few
synthetic rows directly:

```sql
INSERT INTO dmarc_report_entries (
  message_id, record_number, report_org_name, domain,
  policy, subdomain_policy, align_dkim, align_spf, pct,
  source_ip, esp, source_host, source_domain,
  city, state, country, reverse_lookup,
  message_count, disposition, eval_dkim, eval_spf,
  header_from, envelope_from, envelope_to,
  auth_dkim_domain, auth_dkim_selector, auth_dkim_result,
  auth_spf_domain, auth_spf_scope, auth_spf_result,
  po_reason, po_comment,
  start_date, end_date
) VALUES (
  'demo-message-001', 0, 'google.com', 'example.com',
  'none', 'none', 'r', 'r', 100,
  '209.85.220.41'::inet, 'Google Mail', 'mail-sor-f41.google.com', 'google.com',
  'Mountain View', 'CA', 'US', ARRAY['mail-sor-f41.google.com'],
  42, 'none', 'pass', 'pass',
  'example.com', 'example.com', '',
  ARRAY['example.com'], ARRAY['s1'], ARRAY['pass'],
  ARRAY['example.com'], ARRAY['mfrom'], ARRAY['pass'],
  ARRAY[]::text[], ARRAY[]::text[],
  EXTRACT(EPOCH FROM now())::bigint - 86400,
  EXTRACT(EPOCH FROM now())::bigint
);
```

Refresh the dashboard and `example.com` appears in the domain list.

To exercise the ingestion code path without AWS, write a small Go test that
opens a sample `.eml` from disk and runs `backend.DmarcReportPrepareAttachment`
+ `backend.DecoderAggregateReport` on it. Real DMARC report samples are
abundant online (search "DMARC aggregate report XML example").

---

## Tests

```sh
# All Go tests
DATABASE_URL='postgres://localhost:5432/dmarc_analyzer?sslmode=disable' \
  go test ./...

# Specific package
go test ./backend/util/...
go test ./backend/handler/...

# With coverage
go test -cover ./...
```

Notes:

- `backend/handler` imports the `db` package, which **requires
  `DATABASE_URL` to be set** even if the test doesn't hit the DB —
  otherwise `db.init()` calls `log.Fatal`. Set it before `go test`.
- `senderbase` tests hit real DNS. They never fail the build because they
  only `fmt.Printf` the result, but they need outbound DNS to be useful.
- `util/domain_test.go` is similarly demonstrative.
- There is no frontend test runner today. `yarn build` runs `vue-tsc`
  type-checks, which catch most regressions.

---

## Regenerating the Database Schema

`backend/schema.sql` is generated, **not hand-written**. The single source
of truth is the GORM model `backend/model/dbmodel.go`.

After modifying the model:

```sh
dropdb --if-exists gen_sql && createdb gen_sql
go run ./backend/cmd/generate_sql.go
echo '-- Code generated by dmarc-analyzer generate_sql. DO NOT EDIT.' > backend/schema.sql
pg_dump -d gen_sql --schema-only --no-owner \
  | sed '/^--/d' \
  | sed '/^SET /d' \
  | sed '/^SELECT /d' \
  | sed 's/public\.//g' \
  | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' -e 's/\n\n*/\n/' \
  >> backend/schema.sql
dropdb --if-exists gen_sql

# apply the new schema to your local dev DB
dropdb --if-exists dmarc_analyzer && createdb dmarc_analyzer
psql -d dmarc_analyzer -f backend/schema.sql
```

`generate_sql.go` uses a hardcoded DSN
`host=localhost dbname=gen_sql sslmode=disable` (no auth), so peer auth
to your local Postgres needs to work. On many Linux distros this means
running as the `postgres` OS user, or editing `pg_hba.conf` to trust
local connections.

**The application does not auto-migrate at runtime.** In your PR
description, document the manual SQL needed to migrate an existing
production database — e.g., `ALTER TABLE dmarc_report_entries ADD COLUMN
foo text;`. The `schema.sql` change is for fresh installs.

---

## Regenerating API Routes & Clients

The Gin route table (`backend/handler/routes.gen.go`) and the per-handler
parameter binders (`backend/handler/handlers.gen.go`) are **generated** from
`api/openapi.json`. So is the TypeScript client at
`frontend/src/services/openapi/`.

### When to regenerate

You changed `api/openapi.json` (added an endpoint, changed a parameter,
adjusted a response schema). Without regen, the server won't expose your
new route and the frontend won't be able to call it.

### Server-side regen

Either:

```sh
./scripts/gen-routes.sh
```

or:

```sh
go generate ./backend/handler
```

Prereq: `openapi-generator` on `$PATH`. Install one of:

- macOS: `brew install openapi-generator`
- Linux (any): write a small shim that runs
  `openapitools/openapi-generator-cli:v7.6.0` in Docker — CI does exactly
  this; see `.github/workflows/openapi-routes.yml`.

The script:

1. Generates `go-gin-server` output to a temp dir.
2. Copies `routers.go` → `backend/handler/routes.gen.go`.
3. Copies `api_default.go` → `backend/handler/handlers.gen.go`.
4. `gofmt -w` both.

### Client-side regen

```sh
cd frontend
yarn openapi:gen
```

This emits a fresh `typescript-axios` client into
`frontend/src/services/openapi/`. **Do not hand-edit** files in that
directory — your changes will be wiped next regen.

### Drift check

CI runs both:

- `backend/handler/openapi_test.go` parses the JSON spec and asserts every
  path/method has a route and vice versa.
- `.github/workflows/openapi-routes.yml` runs `./scripts/gen-routes.sh`
  and `git diff --exit-code`.

So if you change the spec without regenerating, CI fails before merge.

---

## Frontend Workflow

```sh
cd frontend
yarn dev            # dev server with HMR on http://localhost:3000
yarn build          # production build (writes to dist/)
yarn preview        # serve the production build locally
yarn lint           # ESLint (no fix)
yarn lint:fix       # ESLint with --fix
yarn openapi:gen    # regenerate the OpenAPI client
```

`yarn build` runs `vue-tsc -b` first, which performs TypeScript type
checks. Type errors break the build — fix them.

### State / data flow

- **Pinia store** (`stores/dmarcStore.ts`) is the only thing that calls the
  service layer. Views and components consume the store.
- **Service** (`services/dmarcService.ts`) is a thin typed facade over the
  generated client. Don't have views talk to `openapi/` directly — keep
  the typed interface in `dmarcService` so regenerated client breakage
  surfaces in one place.

### Adding a new view

1. Create `frontend/src/views/MyNewView.vue` (PascalCase filename).
2. Register the route in `frontend/src/router/index.ts`.
3. If you need a new backend endpoint, do the OpenAPI dance (above).
4. Add a `fetchXxx` action in the Pinia store.
5. Wire it up.

### Theming

A single light Vuetify theme is defined in `frontend/src/main.ts`
(`dmarcTheme`). Customize colors there.

---

## Common Tasks

### Add a new API endpoint

1. Add the path / method / schemas to `api/openapi.json`.
2. `./scripts/gen-routes.sh` (regenerates `routes.gen.go`, `handlers.gen.go`).
3. Write the handler in `backend/handler/handler.go`. Function name must
   match `operationId` (e.g. `HandleFooBar`). Use the generated
   `GetHandleFooBarParams(c)` helper to read params.
4. `cd frontend && yarn openapi:gen` to refresh the TS client.
5. Add a wrapper in `frontend/src/services/dmarcService.ts`.
6. Update the consumer (view / store).
7. Update [`docs/API.md`](API.md).

### Add a new column to the schema

1. Add the field to `backend/model/dbmodel.go`.
2. Regenerate `backend/schema.sql` (see above).
3. Populate the field where appropriate in `backend/process.go::ParseDmarcReport`.
4. Surface the field in the relevant handler (`SELECT ...` + response struct).
5. Update the OpenAPI spec if the field appears in the API response.
6. Regenerate client.
7. Document the migration SQL in your PR description.

### Add a new ESP heuristic

Edit `backend/senderbase/senderbase.go` — the `if sbGeo.ESP == "" && ...`
block is where org-name substring matches live. For IPv6, `GetIPV6Data`
holds the hostname-suffix heuristics.

### Add a new attachment format

Extend `DmarcReportPrepareAttachment` in `backend/process.go`. The pattern
is already a long content-type / filename switch — add another arm.

### Change the server port

`backend/cmd/server/server.go`'s `http.ListenAndServe(":6767", r)` is
hardcoded. If you change it, update the Dockerfile `EXPOSE`,
`docker-compose.yml`, and `frontend/vite.config.ts` proxy target.

---

## Debugging Tips

### "DATABASE_URL not set in environment" on startup

`backend/db/db.go::init()` is called on import, before `main`. Set
`DATABASE_URL` even for utilities and tests that don't actually touch the DB.

### Consumer doesn't process messages

1. Is the SQS message even firing? Check `Messages available` on the SQS
   console after dropping a test file into S3.
2. Does the IAM principal have `sqs:ReceiveMessage` + `sqs:DeleteMessage`?
3. Are `AWS_REGION` / `SQS_QUEUE_URL` set correctly in the consumer's env?
4. Did the consumer log `Starting SQS message consumer...`? If not, it
   crashed early — likely a missing env var.

### Parsing fails on a real DMARC report

Save the offending email to a file. Write a one-off test:

```go
func TestRealReport(t *testing.T) {
    f, _ := os.Open("/tmp/sample.eml")
    defer f.Close()
    r, err := DmarcReportPrepareAttachment(f)
    if err != nil { t.Fatal(err) }
    rep, err := DecoderAggregateReport(r)
    if err != nil { t.Fatal(err) }
    t.Logf("%+v", rep)
}
```

This bypasses S3 / SQS and isolates the format issue. If the format is
genuinely new, add a branch to `DmarcReportPrepareAttachment`.

### Frontend can't reach the backend

In dev: Vite proxies `/api` to `http://127.0.0.1:6767`
(see `vite.config.ts`). Make sure the server is actually running on that
port.

In production: the Go server serves both the SPA and the API on `:6767`
(same origin). If you split them, you'll need to set the client's
`basePath` in `dmarcService.ts` to the API origin, and likely tighten CORS
on the server.

### Chart shows wrong dates

The chart bucket math in `getDmarcDailyAll` uses **86000** seconds per day
(not 86400). This is intentional / historical; the frontend normalizes via
`new Date(timestamp)` and rounds to date strings, so display is correct.
Don't "fix" the 86000 without understanding why it's there.

---

## CI Overview

Two workflows in `.github/workflows/`:

### `openapi-routes.yml`

Runs on PRs and pushes to `main`. Installs a Docker-backed
`openapi-generator` shim, runs `./scripts/gen-routes.sh`, then
`git diff --exit-code`. Fails if the generated files would have changed —
forcing you to commit the regen.

### `docker-build.yml`

Runs on push to `main`, on tag pushes matching `v*`, and on releases.
Builds linux/amd64 and linux/arm64 in parallel (using both standard and
ARM runners), publishes to `ghcr.io/${{ github.repository }}`. Tags are
derived from `docker/metadata-action` — branch name, `latest`, and SHA.

---

## Commit & PR Conventions

Mirrored from [`CONTRIBUTING.md`](../CONTRIBUTING.md):

- Imperative, lowercase, prefixed when natural: `fix:`, `feat:`, `docs:`,
  `ci:`, `refactor:`, `chore:`, `test:`.
- Sign-off and GPG-sign: `git commit -s -S -m "fix: ..."`.
- Don't bundle unrelated changes.
- Update docs in the same PR as the behaviour change.
- Add tests where reasonable.
- Provide before/after screenshots for visible frontend changes.

CI must pass before merge.
