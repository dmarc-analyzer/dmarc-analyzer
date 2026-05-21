# Architecture

This document explains how DMARC Analyzer is structured, how data flows through
it, and why each piece exists. Read this when you want to change the system,
debug something past the surface, or just understand what's actually happening
between SES and the dashboard.

For a one-screen overview see the diagram in the [README](../README.md). For
the API contract see [`API.md`](API.md). For deployment specifics see
[`DEPLOYMENT.md`](DEPLOYMENT.md).

---

## Table of Contents

- [System Context](#system-context)
- [Process Topology](#process-topology)
- [End-to-end Data Flow](#end-to-end-data-flow)
- [Backend Packages](#backend-packages)
- [Database Schema](#database-schema)
- [Enrichment Pipeline](#enrichment-pipeline)
- [API Layer & Code Generation](#api-layer--code-generation)
- [Frontend Architecture](#frontend-architecture)
- [Backfill Tool](#backfill-tool)
- [Idempotency & Failure Handling](#idempotency--failure-handling)
- [Build & Release Pipeline](#build--release-pipeline)
- [Design Decisions & Trade-offs](#design-decisions--trade-offs)

---

## System Context

```
+--------------------+        +-------------------+         +----------------+
| Mailbox providers  |  rua=  | AWS SES receiving |  store  |  S3 bucket     |
| (Google, MS, etc.) | -----> | (your domain)     | ------> |  raw .eml      |
+--------------------+        +-------------------+         +-------+--------+
                                                                    | s3:ObjectCreated
                                                                    v
                                                            +-------+-------+
                                                            |  SQS queue    |
                                                            +-------+-------+
                                                                    |
                                                                    v
                          +---------+    enrich     +-----------+   |
                          |  DNS    |<-------------+ consumer  |<--+
                          |+ SBase  |              |  (Go)     |
                          +---------+              +-----+-----+
                                                         | INSERT
                                                         v
                                                 +-------+-------+
                                                 | PostgreSQL    |
                                                 +-------+-------+
                                                         ^
                                                         | SELECT
                                                         |
                                                  +------+-------+        +-----------+
                                                  |   server     |<------>|  browser  |
                                                  |  Gin + SPA   |  HTTPS |  Vue SPA  |
                                                  +--------------+        +-----------+
```

External dependencies the analyzer talks to:

| Dependency | Direction | Why |
|------------|-----------|-----|
| AWS S3 | read (`GetObject`, `ListObjectsV2`) | fetch raw report emails |
| AWS SQS | read + delete (`ReceiveMessage`, `DeleteMessage`) | event-driven ingestion |
| Public DNS resolvers | read (PTR, TXT) | reverse DNS + SenderBase lookups |
| Public Suffix List | static (compiled in) | derive organisational domain |
| PostgreSQL | read + write | persistence |

Note: the analyzer never sends mail and never writes to S3 or SQS in
production. SES delivers; the analyzer only consumes.

---

## Process Topology

The repo produces **three Go binaries from one Dockerfile**:

| Binary | Source | Lifecycle | Purpose |
|--------|--------|-----------|---------|
| `server` | `backend/cmd/server/server.go` | long-running | HTTP API + static SPA |
| `consumer` | `backend/cmd/consumer/consumer.go` | long-running | SQS poll loop |
| `backfill` | `backend/cmd/backfill/backfill.go` | one-shot | Scan S3, import history |

`generate_sql` (`backend/cmd/generate_sql.go`) is also a binary but is a
developer-only tool — see [Database Schema](#database-schema).

The container's default `CMD` is `./server`. Run other binaries with
`docker run … ./consumer` etc.

You **should** run `server` and `consumer` as separate processes (separate
deployments, separate scaling, separate restart policies):

- `server` is stateless and horizontally scalable — put behind a load balancer.
- `consumer` is also stateless but you usually run a **single replica**
  (multiple replicas just compete on the SQS queue, which is fine, but
  duplicates protection is enforced at the DB layer, not at the queue).

---

## End-to-end Data Flow

A single DMARC report's journey from mailbox provider to dashboard:

1. **Provider sends RUA.** Google (etc.) emails a daily aggregate report to
   the `rua=` address on your `_dmarc.example.com` TXT record. Subject is
   typically `Report Domain: example.com Submitter: google.com Report-ID:
   <uuid>`. Body is a `multipart/mixed` MIME with a `.zip` or `.gz`
   attachment containing one XML file.
2. **SES stores email in S3.** A receiving rule for that recipient address
   writes the full `.eml` to the configured S3 bucket. The object key is
   typically the SES message ID — a 26+ character random string.
3. **S3 emits an event.** The bucket's event notification fires
   `s3:ObjectCreated:Put` (or `Post`) to the SQS queue. The SQS message body
   is JSON of shape `{ "Records": [ { "eventName": "...", "s3": { "object":
   { "key": "..." } } } ] }`.
4. **Consumer long-polls SQS.** `messageprocessor.StartMessageConsumer` runs
   a loop with `ReceiveMessage` (`MaxNumberOfMessages: 10`, `WaitTimeSeconds:
   20`, `VisibilityTimeout: 30`).
5. **Deduplication.** For each S3 object key in the event, the consumer
   `SELECT COUNT(*)` from `dmarc_report_entries WHERE message_id = ?`. If
   non-zero, skip (idempotency — see [Idempotency](#idempotency--failure-handling)).
6. **Fetch the email.** `s3client.S3Client.GetObject(bucket, key)`.
7. **Unwrap the attachment.** `DmarcReportPrepareAttachment` walks the MIME
   structure looking for the report attachment. It handles:
   - `multipart/*` — iterate parts, find one of the formats below.
   - `application/gzip`, `application/x-gzip`, `application/gzip-compressed`,
     `application/gzipped`, `application/x-gunzip`,
     `application/x-gzip-compressed`, `gzip/document` — base64 decode then
     `gzip.NewReader`.
   - `application/zip`, `application/x-zip-compressed` — base64 decode then
     `zip.NewReader`, return the first file.
   - `text/xml` — pass through as-is.
   - `application/octet-stream` with `.zip` / `.gz` filename suffix — same as
     above based on suffix.
8. **Parse XML.** `DecoderAggregateReport` runs `xml.NewDecoder` with
   `charset.NewReaderLabel` so non-UTF-8 reports (rare but real) decode
   correctly. The result is a `model.AggregateReport` matching DMARC
   aggregate report XML.
9. **Per-record enrichment.** `ParseDmarcReport` iterates each record:
   - Reverse-DNS the source IP (`net.LookupAddr`).
   - SenderBase TXT lookup for org name, host, domain, city/state/country,
     lat/long (or IPv6 fallback that just does PTR + a small allowlist).
   - Flatten DKIM / SPF authentication result arrays into parallel string
     slices (PostgreSQL `text[]` columns).
   - Project policy_published fields from the parent into each record.
10. **Bulk insert.** `db.DB.Create(reports)` writes all records for the email
    in one statement. Composite primary key `(message_id, record_number)`
    enforces uniqueness.
11. **Delete the SQS message.** Only on a successful insert — otherwise the
    message goes back to the queue after `VisibilityTimeout` for retry.

Reading flow (UI):

1. Browser loads `GET /` from the server, which serves the Vite-built SPA
   from `./frontend/dist`.
2. SPA calls `GET /api/domains` to populate the domain list view.
3. User clicks a domain → SPA navigates to `/report/<domain>/<start>/<end>`,
   fires `GET /api/domains/{domain}/report` and
   `GET /api/domains/{domain}/chart/dmarc`.
4. User clicks a row in the summary table → SPA fires
   `GET /api/domains/{domain}/report/detail?source=…&source_type=…` for the
   drill-down dialog.

---

## Backend Packages

### `backend/` (root package)

`process.go` is the heart of ingestion:

- `ParseNewMail(messageID)` — high-level entry: S3 GetObject →
  PrepareAttachment → DecoderAggregateReport.
- `DmarcReportPrepareAttachment(r)` — the big "format zoo" switch described
  above. **This is where format compatibility lives** — if a provider sends
  a new content-type variant, this is the function to extend.
- `DecoderAggregateReport(r)` — XML decode with charset detection.
- `ExtractZipFile(r)` — helper for the zip branch.
- `ParseDmarcReport(feedback, messageID)` — XML → DB rows + enrichment.
- `ResolveAddrNames(addr)` — convenience wrapper for `net.LookupAddr`.

Note: `init()` functions in `s3client`, `sqsclient`, `db` mean that
**importing those packages performs side effects** (read env, dial DB,
build AWS clients). This is why `backfill.go`, which doesn't need SQS, only
imports `s3client` and `db`.

### `backend/cmd/server/`

`server.go` builds a Gin engine with:

- Permissive CORS (`AllowAllOrigins`) — fine because all endpoints are read-only;
  consider tightening if you put the API on a different origin in production.
- `gin.Recovery()` panic catcher.
- `handler.RegisterRoutes(r)` — registers the four API routes from generated
  code.
- `static.Serve("/", static.LocalFile("./frontend/dist", false))` — serves
  the SPA.
- A catch-all `GET /report/*path` that serves `index.html` for SPA deep links
  (Vue Router uses `createWebHistory`, so any client-side route that's not
  the root needs a server-side fallback).

The server listens on `0.0.0.0:6767` (hardcoded). To change the port, edit
`server.go` or put a reverse proxy in front.

### `backend/cmd/consumer/`

`consumer.go` is intentionally thin — it just wires up:

- A cancellable `context.Context`.
- `signal.Notify(sigChan, SIGINT, SIGTERM)` for graceful shutdown.
- `go messageprocessor.StartMessageConsumer(ctx)`.
- Block on a signal, cancel the context, log, exit.

All the real work is in `messageprocessor`.

### `backend/cmd/backfill/`

`backfill.go` is a one-shot import: it paginates `ListObjectsV2` over the
configured S3 bucket and for each object key that's not already in
`dmarc_report_entries` runs the same `ParseNewMail` → `ParseDmarcReport` →
`db.Create` pipeline as the consumer.

It's safe to run repeatedly; the `message_id` dedupe ensures already-imported
objects are skipped.

It does **not** delete S3 objects or interact with SQS.

### `backend/cmd/generate_sql.go`

A developer-only tool. Connects to a throwaway local database called
`gen_sql`, runs `gorm.AutoMigrate(&model.DmarcReportEntry{})`, and lets you
`pg_dump` the schema. The generated SQL is what's committed at
`backend/schema.sql`. See [Database Schema](#database-schema).

### `backend/handler/`

- `handler.go` — hand-written business logic for all four endpoints
  (`HandleDomainList`, `HandleDomainSummary`, `HandleDmarcDetail`,
  `HandleDmarcChart`). Reads parameters via `GetHandleXxxParams(c)` helpers
  which are generated.
- `handlers.gen.go` — generated bindings between Gin's `gin.Context`
  (query/path params) and the typed `HandleXxxParams` structs. Validates
  presence of required path params and returns `400` if missing.
- `routes.gen.go` — generated `Routes` slice + `RegisterRoutes(r)`. Methods
  + paths come straight from `api/openapi.json`.
- `generate.go` — contains the lone `//go:generate ../../scripts/gen-routes.sh`
  directive, so `go generate ./backend/handler` regenerates both files.
- `openapi_test.go` — a small test that loads `api/openapi.json` and
  verifies every spec route exists in `routes.gen.go` and vice versa. CI runs
  this in `openapi-routes.yml` as belt-and-suspenders.

### `backend/messageprocessor/`

Owns the SQS poll loop:

- `StartMessageConsumer(ctx)` — `ReceiveMessage` with long polling
  (`WaitTimeSeconds: 20`), 30 s visibility, batches of 10. On error, sleep 5
  s and continue.
- `ProcessMessage(msg)` — JSON-decode the SQS body into an `S3Event`, then
  for each `S3EventRecord` that matches `ObjectCreated:Put` or
  `ObjectCreated:Post`:
  1. Dedup-check by `message_id`.
  2. `backend.ParseNewMail` → `backend.ParseDmarcReport` → `db.Create`.
  3. Continue (don't propagate) on any per-record error so a poisoned message
     doesn't block the batch.
- `DeleteMessage` only fires after `ProcessMessage` returns nil for the
  whole SQS message. Per-record errors are logged but don't fail the batch.

### `backend/model/`

Three files:

- `dbmodel.go` — `DmarcReportEntry` GORM struct that maps 1:1 to the
  `dmarc_report_entries` table.
- `dbtype.go` — two custom GORM types:
  - `StringArray` — `[]string` ↔ Postgres `text[]` (custom `Scan` / `Value`,
    `GormDataType() = "text[]"`).
  - `Inet` — `net.IP` ↔ Postgres `inet` (custom `Scan` / `Value`,
    `GormDataType() = "inet"`).
- `xmlmodel.go` — `AggregateReport`, `AggregateReportRecord`, `POReason`,
  `DKIMAuthResult`, `SPFAuthResult`. Mapped to the DMARC aggregate report XSD.

### `backend/db/`

`db.go` opens a single GORM connection in `init()` from `DATABASE_URL` and
exposes the global `db.DB`. **Will `log.Fatal` if `DATABASE_URL` isn't set.**

This is the single place that touches Postgres connection setup. Everything
else uses `db.DB`.

### `backend/s3client/`, `backend/sqsclient/`

Each package has an `init()` that:

1. Calls `config.LoadDefaultConfig(ctx)` — picks up creds from env vars, the
   shared config file, or the instance/container role.
2. Builds the typed AWS SDK v2 client.
3. Reads the bucket name / queue URL from env.

The SDK calls are not in `init()` — they're triggered on actual usage.

### `backend/senderbase/`

SenderBase is Cisco's IP reputation system, queried via DNS TXT:

- `byteReverseIP4(ip)` — produces the reversed-octet form expected by the
  `*.query.senderbase.org` zone.
- `SenderbaseIPData(sip)` — for IPv4, builds `1.2.3.4.query.senderbase.org`
  (reversed) and runs `net.LookupTXT`. TXT records are split by `|` and
  parsed by numeric key (`1=OrgName`, `4=OrgID`, `5=OrgCategory`,
  `20=Hostname`, `21=DomainName`, `50=City`, `51=State`, `53=Country`,
  `54=Longitude`, `55=Latitude`).
- ESP fallback heuristics: matches the org name against `google`, `gmail`,
  `amazon`, `aws`, `mailchimp` substrings (case-insensitive).
- `GetIPV6Data(sip)` — no SenderBase IPv6 zone; falls back to `LookupAddr`
  plus suffix-matching for `outlook.com` (→ Outlook ESP) and Google's
  `unverified-forwarding.1e100.net`.

### `backend/util/`

- `domain.go::GetOrgDomain(domain)` — uses `golang.org/x/net/publicsuffix` to
  return the registrable domain ("organisational domain" in DMARC terms).
- `domain.go::GetESP(orgDomain)` — small static map. Currently unused by the
  ingest path (which has its own logic), kept for callers that want a quick
  org-domain → ESP guess.
- `date.go::ParseDate(start, end)` — accepts either RFC3339Nano or
  `YYYY-MM-DD`. Defaults: start = now - 30 days, end = now. Clamps end to
  now if a future date is passed. End-of-day handling: for `YYYY-MM-DD` the
  end is set to 23:59:59 UTC of that day; for RFC3339 the value is used as-is.

---

## Database Schema

Single table, composite primary key, no foreign keys:

```sql
CREATE TABLE dmarc_report_entries (
    message_id          text NOT NULL,    -- S3 object key for the source email
    record_number       bigint NOT NULL,  -- index of this record inside the email's XML
    report_org_name     text,             -- e.g. "google.com"
    domain              text,             -- the reported-on domain (your domain)
    policy              text,             -- p=
    subdomain_policy    text,             -- sp=
    align_dkim          text,             -- adkim=
    align_spf           text,             -- aspf=
    pct                 bigint,           -- pct=
    source_ip           inet,
    esp                 text,             -- resolved ESP, e.g. "Google Mail"
    org_name            text,             -- SenderBase org name
    org_id              text,             -- SenderBase org id
    source_host         text,             -- SenderBase hostname
    source_domain       text,             -- SenderBase organisational domain
    city, state, country text,
    longitude, latitude text,
    reverse_lookup      text[],           -- PTR records
    message_count       bigint,           -- DMARC <count>
    disposition         text,             -- none|quarantine|reject
    eval_dkim           text,             -- pass|fail
    eval_spf            text,             -- pass|fail
    header_from         text,
    envelope_from       text,
    envelope_to         text,
    auth_dkim_domain    text[],
    auth_dkim_selector  text[],
    auth_dkim_result    text[],
    auth_spf_domain     text[],
    auth_spf_scope      text[],
    auth_spf_result     text[],
    po_reason           text[],
    po_comment          text[],
    start_date          bigint,           -- Unix seconds — DMARC date_range begin
    end_date            bigint,           -- Unix seconds — DMARC date_range end
    last_update         text,
    PRIMARY KEY (message_id, record_number)
);
```

Notes:

- `message_id` is the S3 object key; `record_number` is `i` from
  `for i, record := range feedback.Records`. The two together are unique
  within the dataset.
- Dates are stored as **Unix seconds** in `bigint`. The choice was probably
  driven by the existing chart bucket math; if you ever migrate, prefer
  `timestamptz`.
- All authentication result arrays are positionally aligned (i.e.
  `auth_dkim_domain[i]`, `auth_dkim_selector[i]`, `auth_dkim_result[i]`
  belong to the same DKIM result entry).

### Regenerating the schema

`backend/schema.sql` is **generated**, not hand-written, so the GORM model
and the SQL stay in sync:

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
```

The `generate_sql.go` binary opens a hardcoded DSN
`host=localhost dbname=gen_sql sslmode=disable` (no auth), so you need a
local Postgres listening on the default unix socket + matching peer auth.

---

## Enrichment Pipeline

Each record in an XML report becomes one row in `dmarc_report_entries`. The
fields added beyond what's in the XML:

| Field | Source | Notes |
|-------|--------|-------|
| `reverse_lookup` | `net.LookupAddr(source_ip)` | All PTR records for the IP |
| `esp` | SenderBase TXT (key `1=OrgName`) → substring match | "Google Mail", "Amazon SES", "MailChimp", "Outlook" |
| `org_name` | SenderBase TXT key `1` | |
| `org_id` | SenderBase TXT key `4` | |
| `source_host` | SenderBase TXT key `20` (lowercased) | |
| `source_domain` | SenderBase TXT key `21` (lowercased) | |
| `city, state, country` | SenderBase TXT keys `50, 51, 53` | |
| `longitude, latitude` | SenderBase TXT keys `54, 55` | |

For IPv6 there's a fallback (`GetIPV6Data`) — Hostname from PTR, ESP only for
`outlook.com` and `unverified-forwarding.1e100.net`.

The enrichment intentionally **fails open** — if SenderBase doesn't respond
or reverse DNS fails, the row is still inserted with empty enrichment
fields. The UI's source labelling (`DmarcReportingDefault.Label()` in
`handler.go`) degrades gracefully:

```
preferred → ESP → SourceDomain → SourceHost → SourceIP (last resort)
```

---

## API Layer & Code Generation

The API is **spec-first**. `api/openapi.json` is the source of truth; both
the Go route table + parameter binders **and** the TypeScript axios client
are generated from it.

```
                  ┌──────────────────────────┐
                  │   api/openapi.json       │   ← source of truth
                  └──────────┬───────────────┘
                             │
              ┌──────────────┴──────────────┐
              │                             │
              ▼                             ▼
   scripts/gen-routes.sh         frontend/package.json
   (openapi-generator            "openapi:gen" script
    go-gin-server)               (typescript-axios)
              │                             │
              ▼                             ▼
   backend/handler/routes.gen.go   frontend/src/services/openapi/
   backend/handler/handlers.gen.go     (api.ts, base.ts, common.ts,
                                        configuration.ts, index.ts)
```

### Server side

- `scripts/gen-routes.sh` uses the `go-gin-server` generator with custom
  templates in `api/openapi-generator/`. It produces a temp directory then
  copies just `routers.go` → `backend/handler/routes.gen.go` and
  `api_default.go` → `backend/handler/handlers.gen.go`, gofmt'd.
- `backend/handler/generate.go` has `//go:generate ../../scripts/gen-routes.sh`
  so `go generate ./backend/handler` works too.
- The generator either uses a local `openapi-generator` (e.g. `brew install
  openapi-generator`) or, in CI, a Docker shim that runs
  `openapitools/openapi-generator-cli:v7.6.0`. See
  `.github/workflows/openapi-routes.yml`.

### Client side

- `cd frontend && yarn openapi:gen` regenerates everything in
  `frontend/src/services/openapi/`. The custom template directory lives at
  `api/openapi-generator/typescript-axios-overrides`.
- The generated client is consumed via `frontend/src/services/dmarcService.ts`,
  which creates a `DefaultApi` with `basePath: ''` so requests target the
  same origin (the Go server, which in production serves both API and SPA).

### Drift check

`backend/handler/openapi_test.go` parses `api/openapi.json` at test time and
asserts every spec path/method has a matching `Route` in `routes.gen.go`, and
vice versa. CI also runs `gen-routes.sh` and `git diff --exit-code` so a PR
that changes the spec but forgets to regen fails.

---

## Frontend Architecture

```
src/
├── App.vue                — Vuetify shell (app bar + <router-view/>)
├── main.ts                — bootstraps Pinia, Router, Vuetify, mounts #app
├── router/index.ts        — three routes: /, /report/:domain, /report/:domain/:start/:end
├── views/
│   ├── DomainsView.vue    — list + search + click-through
│   └── ReportView.vue     — chart + summary table + DetailDialog
├── components/
│   ├── DateRange.vue      — date pickers with presets
│   ├── LineChart.vue      — Chart.js wrapper for the pass/fail trend
│   └── DetailDialog.vue   — modal for per-source raw rows
├── stores/dmarcStore.ts   — Pinia store: domains, summaryReport, detailReport, loading, error
├── services/
│   ├── dmarcService.ts    — typed facade over the generated client
│   └── openapi/           — generated typescript-axios client (DO NOT EDIT)
└── utils/utilities.ts     — date math, clipboard, session storage, % formatting
```

Key conventions:

- **State** lives in a single Pinia store. The store is the only thing that
  calls `dmarcService`; views and components consume the store.
- **API base path** is empty (`basePath: ''`), so requests are relative to
  the page origin. In dev, Vite's proxy (`vite.config.ts`) forwards `/api`
  to `http://127.0.0.1:6767`. In production, the Go server serves both the
  SPA and the API on the same origin/port.
- **Routing** uses `createWebHistory` (no hash). The Go server has a
  catch-all `GET /report/*path` → `index.html` so deep links work on refresh.
- **Theming** is a single light theme (`dmarcTheme`) configured in `main.ts`.
- **No frontend test harness** is wired up. `vue-tsc` runs during `yarn
  build` so type errors surface there; ESLint runs via `yarn lint`.

Frontend dev server:

```sh
cd frontend
yarn install
yarn dev      # http://localhost:3000, proxies /api to 6767
```

---

## Backfill Tool

`backfill` exists for two scenarios:

1. **First-time bootstrap.** You already have months of historical reports in
   S3 (perhaps SES has been collecting them before the analyzer was deployed)
   and want them in the dashboard.
2. **Disaster recovery.** The Postgres database is gone but the S3 objects
   are still intact — re-import everything.

Implementation: paginate `ListObjectsV2` over the entire bucket (no prefix
filter — set `S3_BUCKET_NAME` to a bucket dedicated to DMARC reports), and
for each key not already in `dmarc_report_entries` run the same pipeline as
the consumer.

There's no rate-limiting on DNS lookups, so for a very large bucket (hundreds
of thousands of objects) consider running it during off-peak hours or adding
a small `time.Sleep` between iterations.

Backfill **does not** care about SQS — it talks straight to S3 and Postgres.
You can leave the consumer running while you backfill; idempotency prevents
double-inserts.

---

## Idempotency & Failure Handling

**Idempotency** is enforced in one place: a pre-insert
`SELECT COUNT(*) FROM dmarc_report_entries WHERE message_id = ?`. Because S3
object keys are unique per email, and we use them as `message_id`, redelivery
of the same SQS message (or backfill running while the consumer is also
running) is a no-op.

If the consumer crashes between parsing and inserting, SQS redelivers after
`VisibilityTimeout` (30 s by default). If it crashes between inserting and
deleting the SQS message, the next delivery sees the row already exists and
skips, then `DeleteMessage` runs.

**Per-record failures don't block the batch.** Inside `ProcessMessage`, an
error parsing one email is logged and the loop moves on. The SQS message is
still deleted after the loop completes — meaning a permanently un-parseable
email will be lost from the queue but will remain in S3 forever. Run
`backfill` periodically (or set up a DLQ on SQS) if you care about that.

**Per-batch failures fail open.** If `ReceiveMessage` returns an error, the
consumer sleeps 5 s and tries again. There's no exponential backoff, no
circuit breaker, no metrics. For production, consider wrapping the loop with
a metrics middleware and a DLQ on the SQS queue.

---

## Build & Release Pipeline

### Dockerfile (multi-stage)

1. `node:22` → install npm deps and `npm run build` the Vue app. Output is
   `/app/dist/`.
2. `golang:1-alpine` → `go mod download`, then build `server`, `backfill`,
   `consumer` as static linux binaries (`CGO_ENABLED=0`).
3. `alpine:latest` → copy the three binaries + the built SPA into
   `/app/frontend/dist/`. `CMD ["./server"]`. CA certs added for HTTPS to
   AWS endpoints.

Note: although the rest of the project documents `yarn`, the Dockerfile uses
`npm install` / `npm run build`. Both work because `package-lock.json` is
committed. Frontend devs can use either locally.

### GitHub Actions

`.github/workflows/docker-build.yml` runs on push to `main`, on tag pushes
matching `v*`, and on releases. It builds linux/amd64 (on `ubuntu-latest`)
and linux/arm64 (on `ubuntu-24.04-arm`) **in parallel**, then merges the
digests into a single multi-arch manifest published as
`ghcr.io/${{ github.repository }}`.

`.github/workflows/openapi-routes.yml` runs on PRs and `main` pushes. It:

1. Installs a shim `openapi-generator` that runs `openapitools/openapi-generator-cli:v7.6.0`
   in Docker — so the workflow doesn't need Java installed.
2. Runs `./scripts/gen-routes.sh`.
3. `git diff --exit-code` — fails the build if the generated files would
   change, i.e. someone edited the spec but forgot to regen.

---

## Design Decisions & Trade-offs

| Choice | Reasoning | Trade-off |
|--------|-----------|-----------|
| **S3 receiving, not Lambda** | Lambda's incoming event for SES doesn't include attachment bodies, only headers. The DMARC payload lives in the attachment, so we must persist the full email first. | Costs a bit more storage; requires the S3 bucket policy for SES. |
| **One row per `(email, record)` pair** | DMARC reports already aggregate per source / disposition; we keep the natural grain. | Some queries (e.g. "all rows for a given source over a month") scan more rows than they would in a star schema. |
| **Single flat table** | Simpler ops; no joins; one set of indexes to think about. | Schema evolution is invasive when adding new XML fields. |
| **Unix-seconds date columns** | Existing chart bucket math is in seconds; kept for compatibility. | Less friendly than `timestamptz` in ad-hoc SQL; the date inputs accept ISO strings to compensate. |
| **GORM with composite key** | Lets the same struct serve as both the write model and the read model. | GORM is heavy for the simple queries we run; some handlers use raw `.Select(...)` strings anyway. |
| **Spec-first OpenAPI** | One artifact, two generators (server + client), one drift check in CI. | Adds an `openapi-generator` dependency to the dev loop. |
| **No auth on the API** | Designed to run behind your VPN / reverse proxy / IAM-protected ALB. | If you expose the server to the public Internet, put auth in front of it (e.g. oauth2-proxy, AWS ALB auth, Cloudflare Access). |
| **CORS `AllowAllOrigins`** | Convenient for local dev. | Tighten in production if the API is exposed cross-origin. |
| **`init()` for AWS / DB setup** | Keeps `main.go` short. | Importing a package side-effects against your environment, which makes tests that import these packages slower / more dependent on env. |
| **Inline enrichment in the hot path** | Reverse DNS + SenderBase TXT happen synchronously per record. | A burst of new emails with a slow resolver can stall the consumer. For high-volume installs, consider an async enrichment job. |
| **No SQS DLQ wired by default** | Keeps the AWS setup simpler for newcomers. | Permanently-broken emails are silently lost from the queue. Add a DLQ on the SQS console if you care. |

---

## What this codebase isn't (yet)

- It does **not** implement a TLSRPT (SMTP TLS Reporting) parser. DMARC
  aggregate reports only.
- It does **not** implement a `ruf=` failure report parser. Aggregate reports
  (`rua=`) only.
- It does **not** support direct IMAP / POP3 mail ingestion — you must use
  AWS SES + S3.
- It does **not** ship with auth, multi-tenant separation, or row-level
  security. Put it behind an authenticated proxy if you need that.
- It does **not** expose metrics (Prometheus, OpenTelemetry) out of the box.
  PRs welcome.
