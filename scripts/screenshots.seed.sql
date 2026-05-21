-- Synthetic data for documentation screenshots.
-- Not committed to production code paths — only used by the demo compose
-- stack at scripts/screenshots.compose.yml.
--
-- Generates 8 domains × 4 sources × 30 days, with a configurable pass rate
-- per domain so the dashboard shows a healthy mix of green / amber / red.

INSERT INTO dmarc_report_entries (
  message_id, record_number, report_org_name, domain,
  policy, subdomain_policy, align_dkim, align_spf, pct,
  source_ip, esp, org_name, source_host, source_domain,
  city, state, country, longitude, latitude,
  reverse_lookup, message_count, disposition,
  eval_dkim, eval_spf,
  header_from, envelope_from, envelope_to,
  auth_dkim_domain, auth_dkim_selector, auth_dkim_result,
  auth_spf_domain, auth_spf_scope, auth_spf_result,
  po_reason, po_comment,
  start_date, end_date
)
SELECT
  'demo-' || dom.d || '-d' || day_offset || '-' || outcome.label AS message_id,
  src.idx AS record_number,
  src.report_org AS report_org_name,
  dom.d AS domain,
  'quarantine'::text AS policy,
  'none'::text AS subdomain_policy,
  'r'::text AS align_dkim,
  'r'::text AS align_spf,
  100 AS pct,
  src.ip::inet AS source_ip,
  src.esp AS esp,
  src.org_name AS org_name,
  src.source_host AS source_host,
  src.source_domain AS source_domain,
  src.city, src.state, src.country, '', '',
  ARRAY[src.source_host]::text[] AS reverse_lookup,
  CASE outcome.label
    WHEN 'pass' THEN (5000 * src.weight * dom.pass_rate)::int + (random() * 200)::int
    ELSE         (5000 * src.weight * (1 - dom.pass_rate))::int + (random() * 40)::int
  END AS message_count,
  'none'::text AS disposition,
  outcome.dkim AS eval_dkim,
  outcome.spf AS eval_spf,
  dom.d AS header_from,
  'bounces.' || dom.d AS envelope_from,
  ''::text AS envelope_to,
  ARRAY[dom.d]::text[] AS auth_dkim_domain,
  ARRAY['s1']::text[] AS auth_dkim_selector,
  ARRAY[outcome.dkim]::text[] AS auth_dkim_result,
  ARRAY['bounces.' || dom.d]::text[] AS auth_spf_domain,
  ARRAY['mfrom']::text[] AS auth_spf_scope,
  ARRAY[outcome.spf]::text[] AS auth_spf_result,
  ARRAY[]::text[] AS po_reason,
  ARRAY[]::text[] AS po_comment,
  EXTRACT(EPOCH FROM now() - (day_offset * INTERVAL '1 day') - INTERVAL '1 day')::bigint AS start_date,
  EXTRACT(EPOCH FROM now() - (day_offset * INTERVAL '1 day'))::bigint AS end_date
FROM (
  VALUES
    ('acme.com',            'google.com', 0.998),
    ('shop.acme.com',       'google.com', 0.95),
    ('blog.acme.com',       'google.com', 0.999),
    ('marketing.acme.com',  'google.com', 0.85),
    ('newsletter.acme.com', 'google.com', 0.74),
    ('example.com',         'google.com', 0.98),
    ('example.org',         'yahoo.com',  0.92),
    ('test.acme.com',       'google.com', 0.50)
) AS dom(d, _ignored, pass_rate)
CROSS JOIN (
  VALUES
    (0, 'google.com',    'Google Mail',           'Google LLC',                 '209.85.220.41', 'mail-sor-f41.google.com',        'google.com',    'Mountain View', 'CA', 'US', 4),
    (1, 'amazonses.com', 'Amazon SES',            'Amazon.com, Inc.',           '54.240.27.7',   'a27-7.smtp-out.amazonses.com',   'amazonses.com', 'Ashburn',       'VA', 'US', 2),
    (2, 'mcdlv.net',     'MailChimp',             'Rocket Science Group, LLC',  '198.2.181.65',  'mail65.smc.mcdlv.net',           'mcdlv.net',     'Atlanta',       'GA', 'US', 1),
    (3, 'unknown',       '',                      '',                           '203.0.113.42',  'mta1.suspicious-host.example',   '',              '',              '',   'XX', 1)
) AS src(idx, report_org, esp, org_name, ip, source_host, source_domain, city, state, country, weight)
CROSS JOIN (
  VALUES
    ('pass', 'pass', 'pass'),
    ('fail', 'fail', 'fail')
) AS outcome(label, dkim, spf)
CROSS JOIN generate_series(0, 29) AS day_offset;

-- Sanity counts
SELECT 'domains' AS what, count(DISTINCT domain) FROM dmarc_report_entries
UNION ALL
SELECT 'rows',    count(*) FROM dmarc_report_entries
UNION ALL
SELECT 'sum_msg', SUM(message_count) FROM dmarc_report_entries;
