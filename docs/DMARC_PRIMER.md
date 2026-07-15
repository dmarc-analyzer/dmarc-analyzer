# DMARC Primer

A short, opinionated explainer of what DMARC is, what aggregate reports
mean, and how to read DMARC Analyzer's dashboard. If you've already
deployed DMARC at a previous job, skip this — you know it.

---

## The 30-second version

> Email's "From:" header has no built-in authentication. Anyone can write
> any value in it. **SPF, DKIM, and DMARC** together give domain owners a
> way to tell receivers "if mail claiming to be from me doesn't pass these
> checks, here's what you should do with it" — and **DMARC aggregate
> reports** are receivers telling you, weekly-ish, what they actually saw.
> DMARC Analyzer parses those reports and shows them to you.

---

## The three protocols

### SPF — Sender Policy Framework

A TXT record at your domain that lists which IPs are allowed to send mail
as your domain. Receivers check the **envelope sender** (aka
`MAIL FROM` / `Return-Path`) against this list.

```
example.com.  IN  TXT  "v=spf1 include:_spf.google.com ip4:198.51.100.0/24 -all"
```

### DKIM — DomainKeys Identified Mail

Senders cryptographically sign outgoing emails. The public key lives in
DNS at `<selector>._domainkey.<sending-domain>`. Receivers verify the
signature.

```
selector1._domainkey.example.com.  IN  TXT  "v=DKIM1; k=rsa; p=MIGfMA..."
```

### DMARC — Domain-based Message Authentication, Reporting & Conformance

A TXT record at `_dmarc.<domain>` that says:

1. **Policy** (`p=`): if neither SPF nor DKIM aligns with the From header,
   what should receivers do?
   - `none` — monitor only, deliver as normal.
   - `quarantine` — put in the spam folder.
   - `reject` — bounce.
2. **Alignment mode** (`adkim=`, `aspf=`): does the SPF / DKIM domain need
   to match the From header **exactly** (`s`, strict) or just by
   organisational domain (`r`, relaxed)?
3. **Reporting addresses**:
   - `rua=mailto:dmarc-reports@example.com` — where to send **aggregate
     reports** (this is what DMARC Analyzer consumes).
   - `ruf=mailto:dmarc-failures@example.com` — where to send **failure
     reports** (per-message, much higher volume, more sensitive — DMARC
     Analyzer does **not** parse these).
4. **Legacy percentage** (`pct=`): RFC 7489 allowed a domain owner to apply
   policy to only a fraction of non-aligned mail. RFC 9989 removed this tag,
   but receivers and reports using the legacy format remain common.

Example record:

```
_dmarc.example.com.  IN  TXT
  "v=DMARC1; p=quarantine; rua=mailto:dmarc-reports@reports.example.com; adkim=r; aspf=r; pct=100"
```

### "Alignment" — the DMARC-specific concept

SPF and DKIM by themselves don't care about the `From:` header. SPF
validates the envelope sender; DKIM validates whatever domain signed the
message. DMARC adds a constraint on top: **the SPF / DKIM domain must
align with the From: header domain.**

- **Relaxed alignment** (`r`): organisational domains must match. So
  `mail.example.com` and `example.com` align.
- **Strict alignment** (`s`): fully-qualified domains must match.

A message **passes DMARC** when at least one of SPF or DKIM is both
authenticated **and** aligned.

---

## What's in an aggregate report?

Every receiver that supports DMARC (Gmail, Microsoft 365, Yahoo, Apple,
etc.) emails a daily-ish XML report to the `rua=` address. The example
below shows the widely deployed RFC 7489-era shape:

```xml
<feedback>
  <report_metadata>
    <org_name>google.com</org_name>
    <email>noreply-dmarc-support@google.com</email>
    <report_id>1234567890</report_id>
    <date_range>
      <begin>1714521600</begin>
      <end>1714608000</end>
    </date_range>
  </report_metadata>
  <policy_published>
    <domain>example.com</domain>
    <adkim>r</adkim>
    <aspf>r</aspf>
    <p>quarantine</p>
    <sp>none</sp>
    <pct>100</pct>
  </policy_published>
  <record>
    <row>
      <source_ip>209.85.220.41</source_ip>
      <count>42</count>
      <policy_evaluated>
        <disposition>none</disposition>
        <dkim>pass</dkim>
        <spf>pass</spf>
      </policy_evaluated>
    </row>
    <identifiers>
      <header_from>example.com</header_from>
    </identifiers>
    <auth_results>
      <dkim>
        <domain>example.com</domain>
        <selector>s1</selector>
        <result>pass</result>
      </dkim>
      <spf>
        <domain>bounces.example.com</domain>
        <result>pass</result>
      </spf>
    </auth_results>
  </record>
  <!-- many more <record> blocks -->
</feedback>
```

Key points:

- RFC 9990 reports use the
  `urn:ietf:params:xml:ns:dmarc-2.0` namespace, omit `pct`, and may include
  fields such as `version`, `generator`, `np`, `testing`, and
  `discovery_method`. DMARC Analyzer accepts both formats and retains a
  warning when it salvages a mixed, unqualified report.
- **One report per (receiver, your-domain, day)**, roughly. Google sends
  one big report per day; some receivers send less often.
- Each `<record>` describes **aggregate** stats for a particular (source
  IP, disposition, alignment) tuple — `<count>` says how many messages
  matched.
- **`policy_evaluated`** is the DMARC-level outcome (after alignment
  check). **`auth_results`** is the raw SPF / DKIM verification result.
- Attachments are typically `.zip` or `.gz`. Some senders use weird
  content types (`gzip/document` etc.) — DMARC Analyzer handles the
  common cases. See `backend/process.go::DmarcReportPrepareAttachment`.

---

## How DMARC Analyzer reads them

The dashboard maps the XML structure to a flat table:

| XML field | DB column | UI shown as |
|-----------|-----------|-------------|
| `report_metadata/org_name` | `report_org_name` | "Receiver" in detail dialog |
| `policy_published/domain` | `domain` | the domain you're viewing |
| `policy_published/p`, `sp`, `adkim`, `aspf`, `pct` | `policy`, `subdomain_policy`, `align_dkim`, `align_spf`, `pct` | not surfaced (use SQL) |
| `record/row/source_ip` | `source_ip` | "Source IP" |
| `record/row/count` | `message_count` | "Total" |
| `record/row/policy_evaluated/disposition` | `disposition` | "Disposition" |
| `record/row/policy_evaluated/dkim` | `eval_dkim` | "Pass DKIM" |
| `record/row/policy_evaluated/spf` | `eval_spf` | "Pass SPF" |
| `record/identifiers/header_from` | `header_from` | "Header From" |
| `record/auth_results/dkim` | `auth_dkim_*` (arrays) | per-row in detail |
| `record/auth_results/spf` | `auth_spf_*` (arrays) | per-row in detail |

Enrichment fields not in the XML (added by the analyzer):

| Column | Source | Purpose |
|--------|--------|---------|
| `reverse_lookup` | `net.LookupAddr(source_ip)` | "Who is this IP?" |
| `esp` | SenderBase + heuristics | "Probably Google Mail" |
| `source_host` | SenderBase | hostname |
| `source_domain` | SenderBase | organisational domain |
| `city`, `state`, `country` | SenderBase | geolocation |
| `longitude`, `latitude` | SenderBase | geolocation |

This is what lets the dashboard group thousands of distinct IPs from
Google's mail farm under a single "Google Mail" row.

---

## How to read the dashboard

### Domains view

Lists every domain you've received reports for in the last 30 days. The
percentage shown is `pass_count / total_count`. **What "passing" means
here**: at least one of SPF / DKIM aligned. (Pure SPF or pure DKIM pass is
fine for DMARC.)

Green ≥ 99%, amber ≥ 90%, red < 90%. These thresholds live in
`frontend/src/utils/utilities.ts::getPercentageColor`.

### Report view (per domain)

- **Line chart** — daily pass vs. fail counts. Look for cliffs (a sender
  suddenly stops being authenticated) and spikes (a spike in fails that
  doesn't correspond to a spike in passes usually means spoofing).
- **Summary table** — rows grouped by `source` (ESP / domain / host / IP),
  ordered by volume. Columns:
  - **Total** — messages in this group.
  - **Passing %** — pass / total in this group.
  - **Pass Both** — both SPF and DKIM aligned.
  - **Pass SPF** — SPF aligned (with or without DKIM).
  - **Pass DKIM** — DKIM aligned (with or without SPF).
- **Click a row** → detail dialog with the underlying records:
  individual `source_ip`, the actual `auth_dkim_domain` / `selector` /
  `result`, etc.

### Reading the data — common patterns

- **A known sender is 100% passing**: nothing to do.
- **A known sender is 100% failing**: their SPF / DKIM is broken — fix
  alignment. Check `auth_dkim_result` / `auth_spf_result` in the detail
  dialog.
- **An unknown sender has high volume**: investigate whether it's a
  legitimate forwarder, a service you forgot you're using, or spoofing.
- **An unknown sender has trivial volume but is failing**: usually
  spoofing background noise — confirm by looking at `country` / `esp`.

### When you can move to `p=quarantine` or `p=reject`

When you can answer "yes" to all of:

- All legitimate senders are 100% authenticated for at least a few weeks.
- You understand every row with non-trivial volume.
- You've added forwarders, marketing tools, and third-party senders to
  your SPF and DKIM.

For legacy deployments that still use RFC 7489's `pct`, move incrementally:
`p=none` → `p=quarantine; pct=10` → `pct=50` → `pct=100` → `p=reject`.
RFC 9989 no longer defines `pct`; follow the capabilities and rollout
guidance of the receivers you depend on. Watch the dashboard at each step.

---

## Reading further

Specifications and primers worth bookmarking:

- [RFC 9989 — DMARC](https://datatracker.ietf.org/doc/html/rfc9989) — the
  current core protocol.
- [RFC 9990 — DMARC aggregate reporting](https://datatracker.ietf.org/doc/html/rfc9990)
- [RFC 7489 — legacy DMARC](https://datatracker.ietf.org/doc/html/rfc7489)
- [RFC 7208 — SPF](https://datatracker.ietf.org/doc/html/rfc7208)
- [RFC 6376 — DKIM](https://datatracker.ietf.org/doc/html/rfc6376)
- [DMARC.org](https://dmarc.org/overview/) — friendly overview.
- [Google's DMARC guide](https://support.google.com/a/answer/2466580) —
  practical for G Suite admins.
- [M3AAWG Best Practices](https://www.m3aawg.org/sites/default/files/m3aawg-dmarc-policies-2017-02-01.pdf) —
  policy rollout playbook.

---

## What DMARC Analyzer does **not** do

- It does not parse **failure (forensic) reports** (`ruf=`). Aggregate
  only.
- It does not parse **TLSRPT** (SMTP TLS Reporting, RFC 8460). Different
  spec.
- It does not send DMARC reports. You're the receiver, not a publisher of
  reports.
- It does not modify your DNS, SPF, or DKIM records. That's all on you.
- It does not score domains or recommend policy moves automatically. The
  data is there; the call is yours.
