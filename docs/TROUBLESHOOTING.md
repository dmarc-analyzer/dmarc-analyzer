# Troubleshooting

Field guide for the most common "it's not working" scenarios. Symptoms
first, then likely causes and how to verify.

If you can't find your issue here, please
[open an issue](https://github.com/dmarc-analyzer/dmarc-analyzer/issues/new)
with logs and the deployment shape (Docker Compose / Kubernetes / etc.).

---

## Table of Contents

- [Dashboard is empty](#dashboard-is-empty)
- [Consumer doesn't start](#consumer-doesnt-start)
- [Consumer starts but never processes a message](#consumer-starts-but-never-processes-a-message)
- [Consumer logs `failed to parse email`](#consumer-logs-failed-to-parse-email)
- [SQS queue depth keeps growing](#sqs-queue-depth-keeps-growing)
- [SQS message redelivery storm](#sqs-message-redelivery-storm)
- [Database errors at server start](#database-errors-at-server-start)
- [`DATABASE_URL not set in environment`](#database_url-not-set-in-environment)
- [Frontend shows old data after deploy](#frontend-shows-old-data-after-deploy)
- [`go test` fails with `connect: connection refused`](#go-test-fails-with-connect-connection-refused)
- [Schema diff after regeneration](#schema-diff-after-regeneration)
- [OpenAPI CI check fails](#openapi-ci-check-fails)

---

## Dashboard is empty

**Symptom**: `/api/domains` returns `[]`; the SPA says "No Domains
Available".

**Likely causes**:

1. **No reports have arrived yet.** RUA reports start showing up
   1–3 days after the `_dmarc` TXT record is published. Be patient on
   first deploy.
2. **SES isn't writing to S3.** Check the S3 bucket — are any `.eml`
   objects landing? If yes, ingestion is working; if no, the problem is
   upstream of the analyzer.
3. **The consumer isn't running or isn't processing.** See sections below.
4. **The 30-day window is empty.** `HandleDomainList` only returns
   domains with records in the last 30 days. If your dataset is older,
   query the report endpoint with explicit dates:
   `curl 'http://localhost:6767/api/domains/example.com/report?start=2024-01-01&end=2024-12-31'`.

**Verify**:

```sh
# Are there any rows at all?
psql "$DATABASE_URL" -c 'SELECT count(*) FROM dmarc_report_entries;'

# What's the latest record timestamp?
psql "$DATABASE_URL" -c \
  'SELECT to_timestamp(MAX(end_date)) FROM dmarc_report_entries;'

# Are there S3 objects?
aws s3 ls s3://your-bucket/ --summarize | tail -5
```

---

## Consumer doesn't start

**Symptom**: container exits immediately, or the binary prints something
short and dies.

**Look for**:

- `DATABASE_URL not set in environment` — set the env var; see
  [`DATABASE_URL not set`](#database_url-not-set-in-environment).
- `unable to load SDK config` — printed by `s3client.init` /
  `sqsclient.init`. Most often: `AWS_REGION` not set. The AWS SDK doesn't
  hard-fail on missing creds (it can still try the instance role) but
  region is mandatory.
- Stack trace on a `nil` pointer or panic — almost always a missing env
  var. Run with `-v` or check stderr in the container.

The consumer prints its bucket and queue URL on startup:

```
s3 bucket name is: your-org-dmarc-reports
SQS queue URL is: https://sqs.us-east-1.amazonaws.com/...
Starting DMARC Analyzer SQS Consumer...
Starting SQS message consumer...
```

If you don't see all four lines, the consumer didn't actually finish
initializing.

---

## Consumer starts but never processes a message

**Symptom**: log shows `Starting SQS message consumer...` followed by a
steady stream of `No messages received, continuing to poll...` even though
you know S3 is receiving emails.

**Likely causes**:

1. **S3 event notification isn't wired up.** In the S3 console, go to
   Properties → Event notifications. There should be an SQS destination
   pointing at your queue.
2. **The SQS access policy doesn't allow S3 to send messages.** See
   [`DEPLOYMENT.md`](DEPLOYMENT.md#step-2--create-the-sqs-queue) — the
   queue policy needs the `AllowS3ToSendMessages` statement.
3. **The S3 bucket and the SQS queue are in different AWS regions.** S3
   events can target same-region SQS only.
4. **The IAM identity used by the consumer doesn't have
   `sqs:ReceiveMessage` on the queue.** Test with
   `aws sqs receive-message --queue-url <url>` using the same credentials
   the consumer has.
5. **Messages are stuck in-flight.** If a previous consumer instance died
   mid-processing and `VisibilityTimeout` hasn't expired (default 30 s),
   messages won't be redelivered yet. Wait 30 seconds.

**Verify**:

```sh
# How many messages are available right now?
aws sqs get-queue-attributes \
  --queue-url "$SQS_QUEUE_URL" \
  --attribute-names ApproximateNumberOfMessages \
                    ApproximateNumberOfMessagesNotVisible

# Drop a test object into the bucket; see if the SQS depth goes up.
aws s3 cp test.eml s3://your-bucket/test.eml
```

---

## Consumer logs `failed to parse email`

**Symptom**: messages come in but lines like
`Failed to parse email <messageID>: PrepareAttachment: ...` appear.

**Likely causes**:

1. The email is **not actually a DMARC report**. SES is receiving stuff
   that doesn't have an XML attachment. Either tighten the SES receiving
   rule (recipient filter) or accept that some failures will happen.
2. The DMARC report is in **a format the parser doesn't handle yet**.
   See `backend/process.go::DmarcReportPrepareAttachment` — the list of
   accepted content types is explicit. Real-world senders sometimes use
   unusual variants. PRs welcome.
3. The XML uses **a character encoding the decoder can't handle**.
   `DecoderAggregateReport` already plugs in `charset.NewReaderLabel` so
   this is rare; if it happens, save the offending email and open an
   issue.

**The good news**: failed parses don't block the queue. The consumer
deletes the SQS message after each batch even if individual records
failed — it logs and moves on. So a poison email won't get stuck in a
retry loop.

**To investigate a specific failure**:

1. Pull the offending object out of S3: `aws s3 cp s3://bucket/<key> /tmp/`.
2. Write a small Go test that runs `DmarcReportPrepareAttachment` on the
   file (see [`DEVELOPMENT.md`](DEVELOPMENT.md#parsing-fails-on-a-real-dmarc-report)).
3. The error message + the email's `Content-Type` headers tell you which
   branch failed.

---

## SQS queue depth keeps growing

**Symptom**: `ApproximateNumberOfMessages` in CloudWatch trends upward
indefinitely.

**Likely causes**:

1. **The consumer is down.** Check the consumer container is running and
   its logs show `Starting SQS message consumer...` recently.
2. **The consumer is up but stuck**: usually a slow database write
   (locked table, no connection). Tail logs for repeated
   `Failed to insert reports for message ...` lines.
3. **You're ingesting faster than you can process**, which is unusual at
   DMARC volumes. If true, scale the consumer's resources or split the
   workload across multiple queues. (Just adding consumer replicas
   doesn't necessarily help — SQS distributes fairly but each consumer
   still bottlenecks on DNS lookups.)

---

## SQS message redelivery storm

**Symptom**: same `messageID` is logged as "Processing message" over and
over.

**Cause**: the consumer crashes after parsing but before
`DeleteMessage`. SQS redelivers after the `VisibilityTimeout` (30 s
default).

**Fix**: find the crash. Most often it's a database error that takes
down the whole consumer (it shouldn't — `ProcessMessage` recovers per
message — but a panic outside that scope can). Check stderr.

**Mitigation**: set a small "Maximum receives" on the queue with a DLQ.
The bad message goes to the DLQ after N tries; the loop unsticks itself.

---

## Database errors at server start

**Symptom**: server starts, logs `&{Listen: tcp ... connection refused`}`
or similar from GORM, but doesn't crash.

**Cause**: GORM's `init` returns the error rather than `log.Fatal`-ing.
Look at `backend/db/db.go`:

```go
DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
fmt.Printf("%+v\n", err)
```

It prints the error and proceeds. Your queries will then fail at runtime.

**Verify**: `psql "$DATABASE_URL" -c 'SELECT 1'` from the same host. If
that works, the issue is the host/port DNS resolution from inside the
container — common in Docker Compose if the `postgres` service isn't
healthy yet.

**Fix**: make sure the server depends on Postgres health (in compose:
`depends_on.postgres.condition: service_healthy`).

---

## `DATABASE_URL not set in environment`

The entire codebase imports `backend/db`, including some `*_test.go`
files. Because `db.init()` runs at import time, that env var **must** be
set even when running a unit test that doesn't touch the DB.

Always launch with the env var:

```sh
export DATABASE_URL='postgres://localhost:5432/dmarc_analyzer?sslmode=disable'
go run ./backend/cmd/server/server.go
# or
DATABASE_URL=... go test ./...
```

If you want to run a tool that genuinely doesn't need a DB, point it at a
local Postgres anyway — even one that returns no rows is fine.

---

## Frontend shows old data after deploy

**Symptom**: pushed a new image with frontend changes; refresh shows the
old SPA.

**Cause**: browser is caching the SPA's main JS bundle. Vite cache-busts
filenames (each build emits hashed filenames), but if `index.html` itself
is cached too, you can still see stale content.

**Fix**:

- Hard refresh (Cmd-Shift-R / Ctrl-Shift-R).
- If you have a CDN / reverse proxy: set `Cache-Control: no-cache` on
  `index.html` specifically.
- The Go static server (`gin-contrib/static`) doesn't add aggressive
  cache headers, so this is usually a downstream CDN issue.

---

## `go test` fails with `connect: connection refused`

**Symptom**: `go test ./backend/handler/...` (or similar) fails to dial
Postgres.

**Cause**: package import side effect — `backend/handler` imports `db`,
which dials Postgres on init.

**Fix**: run a Postgres locally first.

```sh
docker run -d --name dmarc-pg \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=dmarc_analyzer -p 5432:5432 \
  postgres:14
psql 'postgres://postgres:postgres@localhost:5432/dmarc_analyzer?sslmode=disable' \
     -f backend/schema.sql
DATABASE_URL='postgres://postgres:postgres@localhost:5432/dmarc_analyzer?sslmode=disable' \
  go test ./...
```

---

## Schema diff after regeneration

**Symptom**: you regenerated `backend/schema.sql` but the diff shows
columns reordered or types subtly different.

**Likely causes**:

1. `gorm.AutoMigrate` ordering: GORM may emit columns in a different
   order than expected. The `pg_dump` filter in the regen recipe tries to
   normalize but can leak ordering changes. Usually safe to commit.
2. Postgres version differences: `pg_dump` from PG 14 vs. 15 can render
   `text` differently in some edge cases. Make sure local Postgres
   matches what the repo targets (14+).
3. Locale differences: rare, but `pg_dump` can emit collation hints. The
   sed filter strips obvious ones; if something slips through, add
   another `sed` arm.

**If the diff is huge**: do the regen in a clean Postgres 14 container to
match CI.

---

## OpenAPI CI check fails

**Symptom**: `.github/workflows/openapi-routes.yml` fails with
`git diff --exit-code` showing changes to `routes.gen.go` /
`handlers.gen.go`.

**Cause**: you edited `api/openapi.json` but didn't commit the
regenerated files.

**Fix**:

```sh
./scripts/gen-routes.sh
git add backend/handler/routes.gen.go backend/handler/handlers.gen.go
git commit -m "ci: regenerate routes after spec change"
```

If you don't have `openapi-generator` installed locally, use the Docker
shim CI uses:

```sh
docker run --rm -u "$(id -u):$(id -g)" -v "$PWD:$PWD" -w "$PWD" \
  openapitools/openapi-generator-cli:v7.6.0 generate \
  -g go-gin-server -i api/openapi.json -o /tmp/gen \
  -t api/openapi-generator \
  --global-property=apis,models=false,supportingFiles=routers.go \
  --additional-properties=packageName=handler,apiPath=go
cp /tmp/gen/go/routers.go backend/handler/routes.gen.go
cp /tmp/gen/go/api_default.go backend/handler/handlers.gen.go
gofmt -w backend/handler/routes.gen.go backend/handler/handlers.gen.go
```
