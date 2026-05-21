# Deployment Guide

This is the new-operator walkthrough: starting from "I have an AWS account
and a domain", ending with "I'm reading DMARC reports in a browser".

The example uses **AWS SES + S3 + SQS** because that's the supported
ingestion path. The application itself is just three Go binaries and a
PostgreSQL database, so you can run it anywhere — locally, on an EC2 box,
inside ECS / EKS, on a Kubernetes cluster, on bare metal — as long as the
binaries can reach AWS and Postgres.

---

## Table of Contents

- [What You'll Build](#what-youll-build)
- [Prerequisites Checklist](#prerequisites-checklist)
- [Step 1 — Create the S3 Bucket](#step-1--create-the-s3-bucket)
- [Step 2 — Create the SQS Queue](#step-2--create-the-sqs-queue)
- [Step 3 — Wire S3 Event Notifications to SQS](#step-3--wire-s3-event-notifications-to-sqs)
- [Step 4 — Configure SES Receiving](#step-4--configure-ses-receiving)
- [Step 5 — Verify the Domain & MX](#step-5--verify-the-domain--mx)
- [Step 6 — Publish the DMARC DNS Record](#step-6--publish-the-dmarc-dns-record)
- [Step 7 — Create IAM Credentials for the App](#step-7--create-iam-credentials-for-the-app)
- [Step 8 — Provision PostgreSQL](#step-8--provision-postgresql)
- [Step 9 — Apply the Schema](#step-9--apply-the-schema)
- [Step 10 — Run the App with Docker Compose](#step-10--run-the-app-with-docker-compose)
- [Optional — Backfill Historical Reports](#optional--backfill-historical-reports)
- [Production Hardening](#production-hardening)
- [Running on Kubernetes](#running-on-kubernetes)
- [Running on ECS / EC2](#running-on-ecs--ec2)
- [Upgrading](#upgrading)
- [Backups](#backups)
- [Smoke Test](#smoke-test)

---

## What You'll Build

You're standing up four things:

1. **An S3 bucket** that SES will write inbound DMARC report emails into.
2. **An SQS queue** that the S3 bucket will notify whenever a new email lands.
3. **SES receiving rules** that route mail sent to a specific address into
   that bucket.
4. **The analyzer itself** — `server`, `consumer`, plus a PostgreSQL
   database.

Mailbox providers (Gmail, Microsoft, Yahoo, etc.) only need to know the
`rua=` mailbox in your `_dmarc` TXT record. Everything else is internal
plumbing.

---

## Prerequisites Checklist

- AWS account with admin (or scoped admin) access.
- A domain you control DNS for (you'll add `_dmarc` TXT and an MX record).
- AWS region that supports **SES email receiving** —
  [check the latest list](https://docs.aws.amazon.com/general/latest/gr/ses.html#ses_inbound_endpoints).
  As of writing: `us-east-1`, `us-west-2`, `eu-west-1`, `eu-central-1`,
  `ap-northeast-1`, etc.
- A way to run Docker (your laptop, an EC2 box, a k8s cluster).
- A PostgreSQL 14+ instance (managed RDS / Aurora, or a self-hosted
  container — both work).

Cost is small: SES inbound is $0.10 / 1,000 emails, S3 is fractions of a
cent, SQS is essentially free at DMARC volumes. The Docker image is free
from GHCR.

---

## Step 1 — Create the S3 Bucket

Console: **S3 → Create bucket**.

- Name: `your-org-dmarc-reports` (must be globally unique).
- Region: pick the SES-receiving region you decided on.
- Block all public access: **leave on**.
- Versioning: optional.
- Default encryption: SSE-S3 (or SSE-KMS if your security team prefers).
- Object lifecycle: optional; you can expire raw `.eml` files after, say,
  90 days once the analyzer has parsed them — the data lives in Postgres
  thereafter. Add a Lifecycle rule that transitions / expires whatever
  prefix SES writes to.

Now add the **bucket policy** that lets SES write into it. Replace
`YOUR_ACCOUNT_ID` and the bucket name:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowSESToWriteEmails",
      "Effect": "Allow",
      "Principal": { "Service": "ses.amazonaws.com" },
      "Action": ["s3:PutObject"],
      "Resource": "arn:aws:s3:::your-org-dmarc-reports/*",
      "Condition": {
        "StringEquals": { "aws:SourceAccount": "YOUR_ACCOUNT_ID" }
      }
    }
  ]
}
```

Apply via **Bucket → Permissions → Bucket policy → Edit**.

---

## Step 2 — Create the SQS Queue

Console: **SQS → Create queue**.

- Type: **Standard**.
- Name: `your-org-dmarc-reports`.
- Configuration:
  - Visibility timeout: **30 seconds** (matches the consumer's expectation).
  - Message retention: **4 days** (default is fine).
  - Receive message wait time: **20 seconds** (long polling — the consumer
    sets this client-side too).
- Access policy: replace the default with the policy below. This grants
  the bucket permission to send messages and your own account broad
  control:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "OwnerFullControl",
      "Effect": "Allow",
      "Principal": { "AWS": "arn:aws:iam::YOUR_ACCOUNT_ID:root" },
      "Action": "SQS:*",
      "Resource": "arn:aws:sqs:YOUR_REGION:YOUR_ACCOUNT_ID:your-org-dmarc-reports"
    },
    {
      "Sid": "AllowS3ToSendMessages",
      "Effect": "Allow",
      "Principal": { "Service": "s3.amazonaws.com" },
      "Action": "sqs:SendMessage",
      "Resource": "arn:aws:sqs:YOUR_REGION:YOUR_ACCOUNT_ID:your-org-dmarc-reports",
      "Condition": {
        "StringEquals": { "aws:SourceAccount": "YOUR_ACCOUNT_ID" }
      }
    }
  ]
}
```

Recommended: create a **dead-letter queue** (DLQ) for messages that
permanently fail (e.g., truly malformed emails). On the main queue, set
"Maximum receives" to ~5 and point at the DLQ. The analyzer doesn't fail
the SQS message on parse error, so DLQ-eligible messages are rare in
practice; this is a safety net.

Copy the **queue URL** from the console — you need it for the app's
`SQS_QUEUE_URL` env var.

---

## Step 3 — Wire S3 Event Notifications to SQS

Console: **S3 → your bucket → Properties → Event notifications → Create
event notification**.

- Name: `dmarc-email-uploaded`.
- Event types: **All object create events** (or specifically
  `s3:ObjectCreated:Put` and `s3:ObjectCreated:Post` — the consumer matches
  both).
- Filter: optional. If your SES rule writes under a prefix (e.g.
  `dmarc-reports/`), set that prefix.
- Destination: **SQS queue → Choose from your queues →** select the queue
  you just made.

S3 will validate the destination policy (which is why we added the
`AllowS3ToSendMessages` statement above) and then write a test message.

---

## Step 4 — Configure SES Receiving

Console: **SES → Email receiving → Rule sets**.

If you don't have an active rule set, **Create rule set** (name it
`default-rule-set` for example) and then **Set as active**.

Inside that rule set, **Create rule**:

- Rule name: `dmarc-reports-to-s3`.
- Recipient condition: add the mailbox you'll use, e.g.
  `dmarc-reports@reports.example.com`. You can list multiple recipients if
  several domains share the analyzer.
- Action: **Deliver to S3 bucket**.
  - S3 bucket: `your-org-dmarc-reports`.
  - Object key prefix: optional (e.g., `incoming/`).
  - Encrypt with KMS: optional.
- (Optional) Add a `Stop rule set` action after the S3 action so other
  actions don't run.

Save and ensure the rule set is **Active**.

---

## Step 5 — Verify the Domain & MX

For SES to receive mail on `reports.example.com` you must:

1. **Verify the domain in SES.** Console: **SES → Verified identities →
   Create identity → Domain →** `reports.example.com`. Follow the DKIM /
   verification TXT instructions.
2. **Point MX records to SES** for that subdomain. In your DNS:

   ```
   reports.example.com.   IN   MX   10   inbound-smtp.us-east-1.amazonaws.com.
   ```

   Substitute the correct region endpoint —
   `inbound-smtp.<region>.amazonaws.com`.

3. If your account is still in the SES sandbox, that **only restricts
   sending**, not receiving. Receiving doesn't require leaving the sandbox.

Wait for DNS to propagate. SES shows "Verified" on the identity page when
ready.

---

## Step 6 — Publish the DMARC DNS Record

In your DNS, add a TXT record at `_dmarc.example.com`:

```
_dmarc.example.com.  IN  TXT  "v=DMARC1; p=none; rua=mailto:dmarc-reports@reports.example.com; fo=1; adkim=r; aspf=r;"
```

Field meanings:

- `p=none` — start in "monitor" mode. Don't reject mail until you understand
  your traffic.
- `rua=mailto:` — where mailbox providers send aggregate reports. **This is
  the address you configured in SES Step 4.**
- `fo=1` — also request a failure report when *either* SPF or DKIM fails
  (DMARC Analyzer does not parse failure reports; you can omit `fo` if
  you don't care).
- `adkim=r`, `aspf=r` — relaxed alignment.

Once you're confident, move to `p=quarantine` then `p=reject`. That's a
DMARC policy decision unrelated to the analyzer.

It takes **24 hours to a few days** before providers start sending you
reports.

---

## Step 7 — Create IAM Credentials for the App

The app needs to:

- `GetObject` and `ListBucket` on the S3 bucket.
- `ReceiveMessage`, `DeleteMessage`, `GetQueueAttributes` on the SQS
  queue.

Create a dedicated IAM user (or role, on EC2 / ECS / EKS):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadDmarcBucket",
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:ListBucket"],
      "Resource": [
        "arn:aws:s3:::your-org-dmarc-reports",
        "arn:aws:s3:::your-org-dmarc-reports/*"
      ]
    },
    {
      "Sid": "ConsumeDmarcQueue",
      "Effect": "Allow",
      "Action": [
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:GetQueueAttributes"
      ],
      "Resource": "arn:aws:sqs:YOUR_REGION:YOUR_ACCOUNT_ID:your-org-dmarc-reports"
    }
  ]
}
```

Generate an access key + secret for the IAM user if you'll run the app
outside AWS. On EC2 / ECS / EKS, attach the policy to the instance / task
role and omit the access keys — the AWS SDK will pick them up
automatically.

---

## Step 8 — Provision PostgreSQL

Anything PostgreSQL 14+ works. Three common options:

| Option | Pros | Cons |
|--------|------|------|
| **Docker Postgres** in the same compose file | Simplest; great for a small home setup | You manage backups; data volume needs care |
| **AWS RDS / Aurora PostgreSQL** | Managed backups, automatic minor upgrades | Costs money; need VPC routing |
| **Self-managed on a VM** | Full control | You manage everything |

The schema is tiny (a single table) but volume scales with the number of
reports × records-per-report. Expect on the order of MBs to a few GBs per
year for a single domain.

The connection string must be a libpq-style DSN, e.g.:

```
postgresql://USER:PASSWORD@HOST:5432/dmarc_analyzer?sslmode=require
```

`sslmode=require` (or stricter) is **strongly recommended** in production.

---

## Step 9 — Apply the Schema

The repo ships `backend/schema.sql` — a single `CREATE TABLE` plus the
composite primary key. Apply it once:

```sh
psql 'postgresql://USER:PASSWORD@HOST:5432/dmarc_analyzer?sslmode=require' \
     -f backend/schema.sql
```

If you're using the Docker Compose example below, `schema.sql` is
volume-mounted under `/docker-entrypoint-initdb.d/` and will apply
automatically the **first time** the Postgres container starts on an empty
volume. If you ever wipe `postgres-data` and bring it back up, it re-runs.

For schema changes (you modified `backend/model/dbmodel.go`), see
[`docs/DEVELOPMENT.md`](DEVELOPMENT.md#regenerating-the-database-schema).

---

## Step 10 — Run the App with Docker Compose

The repo's checked-in `docker-compose.yml` is a quick local dev convenience
and **does not include the consumer**. For real deployments, use a
compose file with all three services:

```yaml
services:
  server:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:main
    command: ["./server"]
    ports:
      - "6767:6767"
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

  consumer:
    image: ghcr.io/dmarc-analyzer/dmarc-analyzer:main
    command: ["./consumer"]
    env_file: .env
    environment:
      DATABASE_URL: postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped

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
    restart: unless-stopped

volumes:
  postgres-data:
```

And the `.env` next to it:

```env
AWS_ACCESS_KEY_ID=AKIA...
AWS_SECRET_ACCESS_KEY=...
AWS_REGION=us-east-1
S3_BUCKET_NAME=your-org-dmarc-reports
SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789012/your-org-dmarc-reports
```

> The `server` service doesn't need the AWS env vars. The `consumer` (and
> `backfill`) do. If you'd rather pass them globally, put them in `env_file`
> for all three.

Start it:

```sh
docker compose up -d
docker compose logs -f consumer
```

Then open <http://localhost:6767>.

**Initially the UI will be empty.** No domains appear until the consumer has
ingested at least one report. Be patient — it takes 1–3 days for the first
RUA reports to arrive after you publish the DNS record.

---

## Optional — Backfill Historical Reports

If your SES rule has been writing to the S3 bucket *before* you deployed
the analyzer, run `backfill` to import everything currently sitting in S3:

```sh
docker compose run --rm \
  -e DATABASE_URL=postgresql://postgres:postgres@postgres:5432/dmarc_analyzer?sslmode=disable \
  --env-file .env \
  server ./backfill
```

`backfill` is idempotent — re-running it does nothing for already-imported
objects. You can also run it side-by-side with the consumer.

---

## Production Hardening

### TLS / reverse proxy

The Go server speaks plain HTTP on port `6767` and has **no auth**. Put it
behind a reverse proxy that does TLS and (preferably) auth:

- **AWS ALB** with HTTPS listener + ACM cert. Add OIDC or Cognito auth on
  the listener if exposing to the Internet.
- **nginx / Caddy / Traefik** in front of the container.
- **Cloudflare Tunnel + Access** for zero-config + identity-aware auth.

### Tighten CORS

`server.go` sets `AllowAllOrigins: true`. If your API is on a separate
origin from the SPA (you're hosting them separately), narrow this. If
they're on the same origin (the default deployment), CORS doesn't matter
anyway.

### Use an instance role

Don't bake `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` into env files in
production. On EC2 use an instance profile, on ECS use a task role, on EKS
use IRSA / Pod Identity. The Go AWS SDK picks them up via
`config.LoadDefaultConfig` automatically.

### Tighten the SQS access policy

The example policy in Step 2 grants `SQS:*` to your account root. In a
locked-down setup, replace with specific actions for the analyzer's IAM
role only:

```json
{
  "Effect": "Allow",
  "Principal": { "AWS": "arn:aws:iam::YOUR_ACCOUNT_ID:role/dmarc-analyzer" },
  "Action": ["sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:GetQueueAttributes"],
  "Resource": "arn:aws:sqs:YOUR_REGION:YOUR_ACCOUNT_ID:your-org-dmarc-reports"
}
```

### Dead-letter queue

Configure a DLQ on the main SQS queue (Maximum receives ≈ 5) so a
permanently-broken message doesn't get silently lost. Hook CloudWatch on
the DLQ depth.

### Postgres SSL

Always set `sslmode=require` (or `verify-full` with a CA bundle) on the
`DATABASE_URL` for production.

### Resource limits

- `server`: ~50–100 MB RAM, single CPU is plenty for most setups.
- `consumer`: ~50–100 MB RAM. DNS lookups dominate latency, not CPU. Don't
  scale `consumer` horizontally unless you're behind on a huge backlog —
  multiple replicas just race on the queue.
- `postgres`: depends on data volume; 1–2 GB RAM is plenty for years of
  reports.

### Log shipping

The binaries log to stdout/stderr in plain text. Pipe Docker logs to your
favourite collector (Loki, CloudWatch Logs, ELK, etc.).

---

## Running on Kubernetes

This repo doesn't ship Helm charts, but the manifests are trivial — two
Deployments and a Service. Sketch:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata: { name: dmarc-server }
spec:
  replicas: 2
  selector: { matchLabels: { app: dmarc-server } }
  template:
    metadata: { labels: { app: dmarc-server } }
    spec:
      containers:
      - name: server
        image: ghcr.io/dmarc-analyzer/dmarc-analyzer:main
        command: ["./server"]
        ports: [{ containerPort: 6767 }]
        env:
        - name: DATABASE_URL
          valueFrom: { secretKeyRef: { name: dmarc, key: DATABASE_URL } }
---
apiVersion: apps/v1
kind: Deployment
metadata: { name: dmarc-consumer }
spec:
  replicas: 1
  selector: { matchLabels: { app: dmarc-consumer } }
  template:
    metadata:
      labels: { app: dmarc-consumer }
      annotations:
        # if using IRSA / Pod Identity, point at your role
        eks.amazonaws.com/role-arn: arn:aws:iam::YOUR_ACCOUNT_ID:role/dmarc-analyzer
    spec:
      serviceAccountName: dmarc-analyzer
      containers:
      - name: consumer
        image: ghcr.io/dmarc-analyzer/dmarc-analyzer:main
        command: ["./consumer"]
        env:
        - name: DATABASE_URL
          valueFrom: { secretKeyRef: { name: dmarc, key: DATABASE_URL } }
        - name: S3_BUCKET_NAME
          valueFrom: { secretKeyRef: { name: dmarc, key: S3_BUCKET_NAME } }
        - name: SQS_QUEUE_URL
          valueFrom: { secretKeyRef: { name: dmarc, key: SQS_QUEUE_URL } }
        - name: AWS_REGION
          value: us-east-1
---
apiVersion: v1
kind: Service
metadata: { name: dmarc-server }
spec:
  selector: { app: dmarc-server }
  ports: [{ port: 80, targetPort: 6767 }]
```

Notes:

- Keep `dmarc-consumer` at `replicas: 1` — the consumer is single-threaded
  by design and extra replicas just race on the queue.
- The `server` is stateless and horizontally scalable.
- Use an Ingress / Gateway resource with TLS in front of the `Service`.
- `backfill` is a one-shot — use a `Job` rather than a `Deployment`.

---

## Running on ECS / EC2

- Build two ECS task definitions: one for `server`, one for `consumer`.
- Set the task role to the IAM role from Step 7.
- Server task: container port 6767, behind an ALB target group.
- Consumer task: no inbound traffic; desired count = 1.
- Backfill: run as a `RunTask` invocation when needed.
- Postgres: usually RDS in this configuration.

Plain EC2: `systemd` unit per binary, instance role for AWS auth, point
`DATABASE_URL` at RDS. Nothing fancy.

---

## Upgrading

The image publishes a rolling `:main` tag (= latest build from the default
branch) plus per-commit `sha-<short>` tags. There is no `:latest` tag —
that's a CI convention this project doesn't use. Recommended:

1. Pin to a specific `sha-<short>` tag in production rather than `:main`.
   See the available tags at
   <https://github.com/dmarc-analyzer/dmarc-analyzer/pkgs/container/dmarc-analyzer>.
2. To upgrade, `docker compose pull && docker compose up -d`.
3. **Check `backend/schema.sql` between versions.** Any schema-affecting
   change ships a new `schema.sql`. The application doesn't auto-migrate —
   you must apply the diff manually. See
   [`docs/DEVELOPMENT.md`](DEVELOPMENT.md#regenerating-the-database-schema).

---

## Backups

Two things to back up:

1. **Postgres** — your dashboard data. Use whatever your DB platform
   provides (RDS automated backups, `pg_dump` cronjob, etc.).
2. **The S3 bucket of raw `.eml` files** — the original source of truth. If
   Postgres dies, you can reconstruct it entirely via `backfill`. Enable
   versioning on the bucket and / or replicate to a second region for
   disaster recovery.

You typically don't need to back up the binaries / config — re-pull the
image and re-set the env vars.

---

## Smoke Test

After deployment, before declaring victory:

1. **Check the consumer logs.** First message should be
   `Starting SQS message consumer...` followed by either `No messages
   received, continuing to poll...` (no reports yet) or `Processing message:
   ...`.
2. **Verify SES → S3 wiring** by sending yourself a test email to the
   `rua=` address from a Gmail account. Within ~30 seconds:
   - A new `.eml` object should appear in the S3 bucket.
   - An SQS message should fire and the consumer log should show
     `Processing message: ...`. Because a plain email isn't a DMARC report,
     parsing will fail with a log like `Couldn't decode attachment ...`. The
     consumer will still delete the SQS message and move on.
3. **Verify queue health** in the SQS console — `Messages available` should
   be at or near zero in steady state. If it's growing, the consumer isn't
   keeping up.
4. **Verify DB writes** with `psql`:
   ```sql
   SELECT count(*) FROM dmarc_report_entries;
   SELECT domain, count(*) FROM dmarc_report_entries GROUP BY domain ORDER BY count DESC;
   ```
5. **Verify the API**:
   ```sh
   curl -s http://localhost:6767/api/domains | jq
   ```
6. **Verify the SPA** at `http://localhost:6767`. The Domains view should
   list whatever you have in Postgres.

If you're stuck, jump to [`TROUBLESHOOTING.md`](TROUBLESHOOTING.md).
