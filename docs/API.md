# API Reference

DMARC Analyzer exposes four read-only JSON endpoints under `/api`. They are
the entire surface area of the server — everything the SPA does is built on
top of these.

The canonical machine-readable contract is **[`api/openapi.json`](../api/openapi.json)**.
That file is the source of truth: Go routes, parameter binders, and the
TypeScript axios client are all generated from it (see
[`docs/ARCHITECTURE.md`](ARCHITECTURE.md#api-layer--code-generation)).

This doc is the human-readable companion — examples, edge cases, and notes
that the schema alone doesn't capture.

---

## Conventions

- **Base URL** in local dev: `http://127.0.0.1:6767`. In production it's
  whatever your reverse proxy / ingress fronts.
- **Authentication**: none built in. The server has no auth layer; put it
  behind an authenticating proxy if you expose it (see
  [`DEPLOYMENT.md`](DEPLOYMENT.md#production-hardening)).
- **CORS**: `AllowAllOrigins`. Adjust in `backend/cmd/server/server.go` if
  needed.
- **Content type**: all responses are `application/json; charset=utf-8`.
- **Date parameters** (`start`, `end`):
  - Preferred format: `YYYY-MM-DD` (UTC).
  - Also accepted: RFC3339Nano (`2026-04-20T00:00:00Z`).
  - If omitted, defaults are `start = now - 30 days`, `end = now`.
  - For `YYYY-MM-DD`, `start` is interpreted as 00:00:00 UTC and `end` as
    23:59:59 UTC of that day.
  - `end` is clamped to `now` if a future timestamp is passed.
- **Errors**: only `400` (missing path param) is returned with a JSON
  body. Most other failures degrade to `200` with an empty/partial result
  and a server-side log line — by design, the dashboard prefers to render
  *something* over an error toast.

---

## Endpoints

### 1. List domains

```
GET /api/domains
```

Returns every domain that has at least one record in the last **30 days**
(this window is hardcoded in `HandleDomainList`).

**Response** — `200 OK`, JSON array of `DomainStat`:

```json
[
  {
    "domain": "example.com",
    "total_count": 12345,
    "pass_count": 12100
  },
  {
    "domain": "marketing.example.com",
    "total_count": 480,
    "pass_count": 422
  }
]
```

Field meanings:

| Field | Type | Notes |
|-------|------|-------|
| `domain` | string | The `policy_published.domain` from the DMARC report. |
| `total_count` | int64 | `SUM(message_count)` across all reports in the 30-day window. |
| `pass_count` | int64 | Count where `eval_dkim = 'pass' OR eval_spf = 'pass'` (i.e. DMARC passes by alignment via at least one mechanism). |

**Example**:

```sh
curl -s http://127.0.0.1:6767/api/domains | jq
```

**Notes**:

- Results are sorted alphabetically by `domain`.
- An empty array means no DMARC data has landed in the last 30 days — not
  necessarily that the system is broken.

---

### 2. Domain summary report

```
GET /api/domains/{domain}/report
GET /api/domains/{domain}/report?start=2026-04-20&end=2026-05-20
```

Aggregates DMARC records for `{domain}` over `[start, end]`, grouped by
**source**. Source is whichever of these is most specific for the record:

```
ESP → SourceDomain → SourceHost → SourceIP
```

If SenderBase identified an Email Service Provider (e.g., "Google Mail"),
that's used. Otherwise the SenderBase organisational domain. Otherwise the
hostname. As a last resort, the raw IP.

The handler caps the result at **1000 rows** sorted by `total_count`
descending.

**Path parameter**:

- `domain` (required) — the reporting domain.

**Query parameters**:

- `start` (optional) — `YYYY-MM-DD` or RFC3339Nano.
- `end` (optional) — `YYYY-MM-DD` or RFC3339Nano.

**Response** — `200 OK`, `DomainSummaryResp`:

```json
{
  "domain": "example.com",
  "start_date": "2026-04-20",
  "end_date": "2026-05-20",
  "domain_summary_counts": {
    "total_count": 12345,
    "pass_count": 12100,
    "spf_aligned_count": 11900,
    "dkim_aligned_count": 11800,
    "fully_aligned_count": 11600,
    "message_count": 12345
  },
  "summary": [
    {
      "source": "Google Mail",
      "source_type": "ESP",
      "total_count": 8000,
      "pass_count": 7950,
      "spf_aligned_count": 7800,
      "dkim_aligned_count": 7900,
      "fully_aligned_count": 7750
    },
    {
      "source": "amazonses.com",
      "source_type": "SourceDomain",
      "total_count": 3200,
      "pass_count": 3150,
      "spf_aligned_count": 3100,
      "dkim_aligned_count": 3120,
      "fully_aligned_count": 3070
    }
    /* ... up to 999 more rows ... */
  ]
}
```

Field meanings on `domain_summary_counts` and each `summary` entry:

| Field | Notes |
|-------|-------|
| `total_count` | Total messages (`SUM(message_count)`). |
| `pass_count` | Messages where at least one of SPF / DKIM aligned (`pass`). |
| `spf_aligned_count` | Messages where SPF eval is `pass` (and DKIM is not, when both pass it goes here too — see source). |
| `dkim_aligned_count` | Messages where DKIM eval is `pass`. |
| `fully_aligned_count` | Messages where **both** SPF and DKIM eval are `pass`. |
| `message_count` | Alias for `total_count` retained for older clients (TODO: deprecate). |

`source_type` is one of:

- `ESP` — `source` is an Email Service Provider label (e.g. "Google Mail").
- `SourceDomain` — `source` is the SenderBase organisational domain.
- `SourceHost` — `source` is the SenderBase hostname.
- `SourceIP` — `source` is a raw IPv4/IPv6 address.

`start_date` / `end_date` are echoed back as `YYYY-MM-DD` strings (UTC).

**Example**:

```sh
curl -s 'http://127.0.0.1:6767/api/domains/example.com/report?start=2026-04-20&end=2026-05-20' \
  | jq '.domain_summary_counts'
```

---

### 3. Domain detail report

```
GET /api/domains/{domain}/report/detail?source=<source>&source_type=<type>
GET /api/domains/{domain}/report/detail?start=<date>&end=<date>&source=<source>&source_type=<type>
```

The drill-down endpoint. Returns up to **2500 raw-ish rows** for a given
`{domain}` filtered to a specific source/source_type, grouped by the
detail columns (so identical rows are summed via `SUM(message_count) AS
count`).

Used by the SPA when the user clicks a row in the summary table.

**Path parameter**:

- `domain` (required) — same as above.

**Query parameters**:

- `start` (optional) — same semantics as endpoint 2.
- `end` (optional) — same semantics.
- `source` (optional) — the value the user clicked from the summary
  (`source` field).
- `source_type` (optional) — one of `ESP`, `SourceDomain`, `SourceHost`,
  `SourceIP`. Determines which column the `source` query targets:

  | `source_type` | Column queried |
  |---------------|----------------|
  | `ESP` | `esp` |
  | `SourceDomain` | `source_domain` |
  | `SourceHost` | `source_host` |
  | `SourceIP` | `source_ip` |

  If `source_type` is omitted or unknown, no source filter is applied and
  you get the full domain detail.

**Response** — `200 OK`, `DmarcDetailResp`:

```json
{
  "detail_rows": [
    {
      "message_count": 4823,
      "report_org_name": "google.com",
      "source_ip": "209.85.220.41",
      "esp": "Google Mail",
      "source_domain": "google.com",
      "source_host": "mail-sor-f41.google.com",
      "reverse_lookup": ["mail-sor-f41.google.com."],
      "country": "US",
      "disposition": "none",
      "eval_dkim": "pass",
      "eval_spf": "pass",
      "header_from": "example.com",
      "envelope_from": "bounces.example.com",
      "envelope_to": "",
      "auth_dkim_domain": ["example.com"],
      "auth_dkim_selector": ["s1"],
      "auth_dkim_result": ["pass"],
      "auth_spf_domain": ["bounces.example.com"],
      "auth_spf_scope": ["mfrom"],
      "auth_spf_result": ["pass"],
      "po_reason": [],
      "po_comment": []
    }
    /* ... */
  ]
}
```

Each row's fields are projections of `dmarc_report_entries` columns plus
the summed `message_count`. See [`docs/ARCHITECTURE.md`](ARCHITECTURE.md#database-schema)
for the column glossary.

**Example**:

```sh
curl -s 'http://127.0.0.1:6767/api/domains/example.com/report/detail?source=Google%20Mail&source_type=ESP&start=2026-04-20&end=2026-05-20' \
  | jq '.detail_rows[0]'
```

**Notes**:

- `disposition` is what the receiver actually did: `none` / `quarantine` /
  `reject`.
- `eval_dkim` / `eval_spf` are the DMARC alignment result, **not** raw SPF /
  DKIM verification result. They're `pass` if the corresponding mechanism
  passed *and* the identifier aligned with the From header.
- `auth_dkim_*` and `auth_spf_*` arrays are positionally aligned: index `i`
  across `auth_dkim_domain`, `auth_dkim_selector`, `auth_dkim_result`
  describes the same DKIM signature.
- `po_reason` / `po_comment` come from `policy_evaluated/reason` — they
  describe policy overrides like "trusted_forwarder", "mailing_list", etc.

---

### 4. DMARC chart data

```
GET /api/domains/{domain}/chart/dmarc
GET /api/domains/{domain}/chart/dmarc?start=2026-04-20&end=2026-05-20
```

Pre-aggregates pass/fail counts into daily buckets across `[start, end]`,
returning the shape the SPA's `LineChart` component eats.

**Path parameter**:

- `domain` (required).

**Query parameters**:

- `start` (optional) — same as above.
- `end` (optional) — same as above.

**Response** — `200 OK`, `DmarcChartResp`:

```json
{
  "domain": "example.com",
  "chartdata": [
    {
      "name": "pass",
      "series": [
        { "name": 1714521600000, "value": 423 },
        { "name": 1714608000000, "value": 467 }
        /* one per day */
      ]
    },
    {
      "name": "fail",
      "series": [
        { "name": 1714521600000, "value": 12 },
        { "name": 1714608000000, "value": 7 }
      ]
    }
  ]
}
```

Field meanings:

- `chartdata` is always exactly two entries: `pass` and `fail`.
- `series[i].name` is a **Unix timestamp in milliseconds** for that day's
  bucket (UTC). The SPA does `new Date(timestamp)` on it.
- `series[i].value` is the message count for that day in that bucket.
- Days with no data are padded with `value: 0` so consumers can render a
  continuous line.

**Example**:

```sh
curl -s 'http://127.0.0.1:6767/api/domains/example.com/chart/dmarc?start=2026-04-20&end=2026-05-20' \
  | jq '.chartdata[].series | length'
```

**Notes**:

- "Pass" here means `eval_dkim = 'pass' OR eval_spf = 'pass'`. "Fail" means
  neither passed.
- The day bucket arithmetic in the backend uses **86000 seconds per day**
  (not 86400) — this is historical but harmless because the frontend
  rounds to date strings; don't "fix" it without verifying the chart
  doesn't regress.

---

## Generating Clients

Because the API is spec-first, you can build clients in any language with
zero hand-coding:

```sh
# Postman / Insomnia: File → Import → api/openapi.json

# OpenAPI Generator — Python example
openapi-generator generate \
  -i api/openapi.json \
  -g python \
  -o ./clients/python

# Same idea for: java, csharp, ruby, php, rust, etc.
```

The TypeScript client used by the SPA is regenerated by
`cd frontend && yarn openapi:gen`.

---

## Versioning

`info.version` in the spec is currently `1.0.0`. Breaking changes (schema
field removed, route removed) bump major; additive non-breaking changes
bump minor. The server doesn't currently version routes (no `/v1/`); a
future breaking change would introduce a parallel `/api/v2/` path.

---

## Rate Limits

There are no built-in rate limits or quotas. Add them at the reverse proxy
layer if you expose the API to a wider audience.
