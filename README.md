# DMARC Analyzer

> Self-hosted DMARC aggregate report parser, store, and dashboard.
> Receive `rua=` reports straight from mailbox providers, decode them, enrich
> with sender intelligence, and visualize who is sending mail "as you".

[![Apache 2.0 License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go 1.25+](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg?logo=go)](https://go.dev/)
[![Vue 3](https://img.shields.io/badge/Vue-3.5-42b883.svg?logo=vue.js)](https://vuejs.org/)
[![PostgreSQL 14+](https://img.shields.io/badge/PostgreSQL-14%2B-336791.svg?logo=postgresql)](https://www.postgresql.org/)
[![Docker Image](https://img.shields.io/badge/image-ghcr.io%2Fdmarc--analyzer%2Fdmarc--analyzer-2496ED.svg?logo=docker)](https://github.com/dmarc-analyzer/dmarc-analyzer/pkgs/container/dmarc-analyzer)

[English](README.md) · [简体中文](README_CN.md)

---

## Table of Contents

- [What is DMARC Analyzer?](#what-is-dmarc-analyzer)
- [Key Features](#key-features)
- [Architecture at a Glance](#architecture-at-a-glance)
- [Tech Stack](#tech-stack)
- [Quick Start (Docker Compose)](#quick-start-docker-compose)
- [Environment Variables](#environment-variables)
- [Documentation](#documentation)
- [API Overview](#api-overview)
- [Project Layout](#project-layout)
- [Contributing](#contributing)
- [License](#license)

---

## What is DMARC Analyzer?

**DMARC** (Domain-based Message Authentication, Reporting & Conformance) lets
domain owners publish a DNS policy describing how receiving mail servers
should treat unauthenticated mail claiming to be from their domain.

Mail providers (Google, Microsoft, Yahoo, Apple, …) then send back **aggregate
RUA reports** — gzipped/zipped XML attachments emailed to the address listed in
the domain's `_dmarc` TXT record. These reports describe how much traffic
"as your domain" each provider saw, whether SPF / DKIM passed, and where it
came from.

The format is open and well-specified, but the reports themselves are:

- delivered as compressed XML attachments to a mailbox,
- emitted by dozens of senders with subtle format variations,
- meaningless to humans without enrichment (raw IP → "who is this actually?").

**DMARC Analyzer** is an end-to-end pipeline that turns that firehose of
attachments into a queryable, browsable dashboard:

1. AWS SES drops every inbound report email into an **S3 bucket**.
2. An **SQS event** notifies the analyzer that a new email has arrived.
3. The **consumer** fetches the email, unwraps the MIME / gzip / zip / XML
   layers, parses the DMARC aggregate report, enriches each row with
   reverse DNS, organisational domain, ESP fingerprint, and geolocation, and
   writes it into **PostgreSQL**.
4. A **Go API server** + **Vue 3 SPA** lets you slice the data per domain, see
   pass / fail trends, drill into individual sources, and find spoofing.

You get a self-hostable, single-binary alternative to SaaS DMARC dashboards —
your reports never leave your AWS account.

---

## Key Features

### Ingestion

- Receives DMARC aggregate reports via **AWS SES → S3 → SQS** event chain.
- Decodes the matrix of attachment formats sent in the wild:
  multipart MIME, base64, gzip (`application/gzip`, `application/x-gzip`,
  `gzip/document`, …), zip (Google / Yahoo styles), bare XML,
  `application/octet-stream` with `.zip`/`.gz` filenames.
- XML decoder with charset auto-detection for non-UTF-8 reports.
- **Idempotent**: each S3 object key (= email message ID) is processed at
  most once even on SQS redelivery.

### Enrichment

- Reverse DNS (PTR) lookups for every source IP.
- Organisational domain extraction using the [Public Suffix List](https://publicsuffix.org/).
- ESP (Email Service Provider) identification — Google Mail, Amazon SES,
  MailChimp, Outlook, Google "unverified forwarding", etc.
- SenderBase (`*.query.senderbase.org`) TXT lookups for org name, hosting
  country, city, lat/long.
- IPv6-aware path with Outlook / Google forwarding heuristics.

### Storage & API

- Single PostgreSQL table with a composite primary key
  `(message_id, record_number)` — see [`backend/schema.sql`](backend/schema.sql).
- Versioned **OpenAPI 3 spec** at [`api/openapi.json`](api/openapi.json).
- Go (Gin) handlers + parameter binders are **code-generated** from the spec
  via `openapi-generator`; a CI check rejects drift.

### Frontend

- **Vue 3 + Vite + Vuetify + Pinia + Chart.js** SPA, served as static files by
  the Go server in production.
- Domain list with per-domain 30-day pass rate.
- Per-domain report view: pass/fail time series, summary by source
  (ESP / domain / host / IP), and per-source drill-down with raw rows.
- Date range picker with presets, deep-linkable via URL.

### Ops

- Multi-arch container image (linux/amd64 + linux/arm64) published to GHCR by
  GitHub Actions.
- Backfill CLI to import historical reports already sitting in S3.
- Graceful shutdown on SIGINT / SIGTERM in the consumer.
- All AWS config via standard environment variables — works with EC2 / EKS /
  ECS instance profiles when you omit access keys.

---

## Architecture at a Glance

```mermaid
flowchart LR
    G[Google / Microsoft<br/>Yahoo / Apple<br/>mailbox providers]:::ext

    subgraph AWS[Your AWS account]
        direction LR
        SES[AWS SES<br/>receiving rule]
        S3[(S3 bucket<br/>raw report emails)]
        SQS[[SQS queue<br/>ObjectCreated events]]
    end

    subgraph App[DMARC Analyzer]
        direction TB
        C[consumer<br/>cmd/consumer]
        BF[backfill<br/>cmd/backfill]
        SRV[server<br/>cmd/server<br/>Gin API + SPA]
        DB[(PostgreSQL<br/>dmarc_report_entries)]
        C -->|parse + enrich| DB
        BF -->|scan S3 + parse| DB
        SRV -->|query| DB
    end

    SB[(SenderBase TXT<br/>+ Reverse DNS<br/>+ Public Suffix List)]:::ext
    U([User / browser]):::user

    G -- "rua= reports" --> SES
    SES -- "store email" --> S3
    S3 -- "s3:ObjectCreated:*" --> SQS
    SQS -- "poll messages" --> C
    C -- "GetObject" --> S3
    BF -- "ListObjects + GetObject" --> S3
    C -. "DNS lookups" .-> SB

    U -- "https://your.domain" --> SRV
    SRV -- "GET / (SPA)" --> U

    classDef ext fill:#fffbe6,stroke:#bfa73a,color:#5a4a00;
    classDef user fill:#e6f7ff,stroke:#3a7abf,color:#003a66;
```

If your renderer doesn't support Mermaid, here's the same flow in ASCII:

```
                +--------------------+
mailbox  rua=   |  AWS SES receive   |
providers ----> |  (email -> S3 rule)|
                +---------+----------+
                          | store email
                          v
                +--------------------+      +-------------------+
                |  S3 bucket         |----->|  SQS queue        |
                |  (raw .eml objects)| event| (ObjectCreated)   |
                +---------+----------+      +---------+---------+
                          ^                           |
                          | GetObject                 | long-poll
                          |                           v
                     +----+----+                +----+---------+
                     | server  |                |   consumer   |
                     | + SPA   |<-- query DB ---+ parse+enrich |
                     +----+----+                +----+---------+
                          |                          |
                          v                          v
                     +----+--------------------------+----+
                     |   PostgreSQL: dmarc_report_entries  |
                     +-------------------------------------+
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for a module-by-module
walkthrough.

---

## Tech Stack

| Layer       | Technology |
|-------------|------------|
| Backend     | Go 1.25, [Gin](https://github.com/gin-gonic/gin), [GORM](https://gorm.io/) |
| Database    | PostgreSQL 14+ (uses `inet` and `text[]` column types) |
| AWS         | [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) — SES (receiving), S3, SQS |
| Frontend    | Vue 3 (Composition API), Vite 7, Vuetify 3, Pinia, Vue Router, Chart.js, date-fns |
| API contract| OpenAPI 3 (`api/openapi.json`); Gin routes + TS axios client are generated |
| Build / CI  | GitHub Actions (multi-arch Docker image, OpenAPI drift check), Docker (multi-stage) |
| Enrichment  | `net.LookupAddr`, `net.LookupTXT`, [Public Suffix List](https://pkg.go.dev/golang.org/x/net/publicsuffix), SenderBase (`*.query.senderbase.org`) |

---

## Quick Start (Docker Compose)

The fastest path is the pre-built image from GHCR plus a local Postgres.

### 1. Prerequisites

- Docker Engine 24+ and `docker compose` (or `docker-compose`).
- AWS account with **SES, S3, SQS** wired up (one-time, see
  [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)).
- A domain whose `_dmarc` TXT record points `rua=mailto:` at an address SES
  receives mail for.

### 2. Create `.env`

```env
# AWS credentials (omit on EC2/ECS/EKS — use the instance role instead)
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1

# DMARC report ingestion
S3_BUCKET_NAME=your-org-dmarc-reports
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/your-org-dmarc-reports
```

### 3. `docker-compose.yml`

Use the example below (it includes the consumer, which the file checked into
the repo currently omits — see issue note in [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)):

```yaml
services:
  server:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:latest
    command: ["./server"]
    ports:
      - "6767:6767"
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy

  consumer:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:latest
    command: ["./consumer"]
    env_file: .env
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:14
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: dmarc_analyzer
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./backend/schema.sql:/docker-entrypoint-initdb.d/schema.sql:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 10

volumes:
  postgres-data:
```

### 4. Launch

```sh
docker compose up -d
docker compose logs -f consumer   # follow ingestion logs
# open http://localhost:6767      # SPA + API on the same port
```

### 5. Backfill (optional)

If your S3 bucket already has historical reports, replay them once:

```sh
docker compose run --rm \
  -e DATABASE_URL=postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable \
  --env-file .env \
  server ./backfill
```

For a step-by-step new-operator walkthrough including IAM, SES, S3 events, and
DNS — see **[`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)**.

---

## Environment Variables

| Variable | Used by | Required | Description |
|----------|---------|----------|-------------|
| `DATABASE_URL` | `server`, `consumer`, `backfill` | yes | PostgreSQL DSN consumed by GORM. Example: `postgresql://user:pass@host:5432/dmarc_analyzer?sslmode=disable` |
| `S3_BUCKET_NAME` | `consumer`, `backfill` | yes | S3 bucket where SES stores inbound report emails. |
| `SQS_QUEUE_URL` | `consumer` | yes (for live ingest) | SQS queue URL subscribed to `s3:ObjectCreated:*` events. |
| `AWS_REGION` | all AWS calls | yes | AWS region for SDK config. |
| `AWS_ACCESS_KEY_ID` | all AWS calls | optional | Omit when running with an instance role (EC2/ECS/EKS). |
| `AWS_SECRET_ACCESS_KEY` | all AWS calls | optional | Same as above. |
| `AWS_SESSION_TOKEN` | all AWS calls | optional | For temporary credentials. |

> ⚠️ Earlier README revisions referenced `DB_HOST` / `DB_PORT` / `DB_USER` /
> `DB_PASSWORD` / `DB_NAME` / `DB_SSLMODE`. The current code only reads
> `DATABASE_URL` (see [`backend/db/db.go`](backend/db/db.go)). Use the DSN form.

---

## Documentation

Detailed docs live under [`docs/`](docs/):

| Doc | Audience | What's in it |
|-----|----------|--------------|
| [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) | engineers | Component diagram, every Go package, data flow, schema, enrichment pipeline, OpenAPI codegen. |
| [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) | new operators | End-to-end AWS setup (SES, S3, SQS, IAM), DNS, env vars, Docker Compose, k8s notes, hardening. |
| [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) | contributors | Local dev loop, tests, OpenAPI regen, schema regen, commit & PR conventions, coding style. |
| [`docs/API.md`](docs/API.md) | API consumers | All four endpoints with parameters, responses, and `curl` examples. |
| [`docs/DMARC_PRIMER.md`](docs/DMARC_PRIMER.md) | newcomers | What DMARC / SPF / DKIM are, why aggregate reports exist, how to read them. |
| [`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md) | operators | Common problems (no data, SQS visibility, parse failures) and how to investigate. |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | contributors | Doorway: PR checklist, commit format, dev environment links. |
| [`AGENTS.md`](AGENTS.md) | repo agents | Conventions for AI / scripted contributors. |

---

## API Overview

The server exposes four read-only JSON endpoints under `/api`. All "date"
parameters accept `YYYY-MM-DD` (preferred) or RFC3339Nano timestamps.

| Method | Path | Purpose |
|--------|------|---------|
| `GET` | `/api/domains` | List domains with last-30-day total + pass counts. |
| `GET` | `/api/domains/{domain}/report?start=&end=` | Summary aggregated by source (ESP / domain / host / IP). |
| `GET` | `/api/domains/{domain}/report/detail?start=&end=&source=&source_type=` | Raw per-source rows for drill-down. |
| `GET` | `/api/domains/{domain}/chart/dmarc?start=&end=` | Pass/fail daily time series for the trend chart. |

Example:

```sh
curl 'http://127.0.0.1:6767/api/domains'
curl 'http://127.0.0.1:6767/api/domains/example.com/report?start=2026-04-20&end=2026-05-20'
```

Full schemas, error semantics, and `curl` examples for each endpoint:
**[`docs/API.md`](docs/API.md)**.

The canonical machine-readable spec is **[`api/openapi.json`](api/openapi.json)** —
import it into Postman, Insomnia, or `openapi-generator` to build clients in
any language.

---

## Project Layout

```
.
├── api/
│   ├── openapi.json                  # OpenAPI 3 spec (source of truth)
│   └── openapi-generator/            # generator template overrides
├── backend/
│   ├── cmd/
│   │   ├── server/server.go          # HTTP API + serves SPA
│   │   ├── consumer/consumer.go      # SQS-driven ingester
│   │   ├── backfill/backfill.go      # one-shot S3 importer
│   │   └── generate_sql.go           # dumps schema.sql via gorm AutoMigrate
│   ├── handler/                      # API handlers + generated routes
│   ├── model/                        # GORM model + XML model + custom types
│   ├── messageprocessor/             # SQS poll loop + dedupe
│   ├── s3client/, sqsclient/         # AWS client init
│   ├── senderbase/                   # IP geolocation / ESP enrichment
│   ├── util/                         # publicsuffix, date parsing
│   ├── process.go                    # MIME / gzip / zip / XML decoding
│   └── schema.sql                    # generated Postgres schema
├── frontend/                         # Vue 3 SPA (Vite + Vuetify + Pinia)
│   └── src/
│       ├── views/                    # DomainsView, ReportView
│       ├── components/               # LineChart, DateRange, DetailDialog
│       ├── stores/                   # Pinia store
│       └── services/openapi/         # generated typescript-axios client
├── scripts/gen-routes.sh             # regenerates routes.gen.go + handlers.gen.go
├── .github/workflows/                # docker-build.yml, openapi-routes.yml
├── docker-compose.yml                # local dev convenience
├── Dockerfile                        # multi-stage: node → go → alpine
├── Makefile                          # build/run/docker targets
└── go.mod
```

---

## Contributing

We welcome PRs, issues, and discussion. Start with:

1. Read **[`CONTRIBUTING.md`](CONTRIBUTING.md)** for the short version.
2. Read **[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md)** for the full dev loop
   (Postgres, Go tests, OpenAPI regen, schema regen, frontend client regen).
3. Browse open issues, or [open a new one](https://github.com/dmarc-analyzer/dmarc-analyzer/issues/new)
   describing your change before sending a large PR.

Quick rules:

- Run `go test ./...` and `cd frontend && yarn build` before pushing.
- Commit messages: short imperative, conventional prefixes when natural
  (`fix:`, `feat:`, `docs:`, `ci:`). Sign-off (`-s`) and GPG-sign (`-S`) where
  possible.
- If you change the API, update [`api/openapi.json`](api/openapi.json) and run
  `./scripts/gen-routes.sh` so the generated Go files match.

---

## License

Licensed under the Apache License, Version 2.0 — see [`LICENSE`](LICENSE).
