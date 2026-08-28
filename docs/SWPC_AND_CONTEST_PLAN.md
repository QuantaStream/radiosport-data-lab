# SWPC and Contest Log Plan

This note defines the next two data families for RadioSport Data Lab:

- historical SWPC A, K/Kp, and SFI/F10.7 values
- adjudicated Cabrillo contest logs, initially limited to Tier 1 Caribbean and
  Central America stations

The goal is not just to load more data. The goal is to make RBN spot activity
joinable to propagation conditions and submitted contest activity.

## Mapper Rule of Thumb

Cardinality is a warning light, not the only decision rule.

Use `StringEnum` for categorical values even when the distinct count grows into
the low thousands, especially when the field is commonly used in dashboards,
`GROUP BY`, `topn()`, equality filters, and dimensional joins. Examples include
band, mode, continent, DXCC prefix, country, category, and status fields.

Use `StringLexBSI` for bounded identifiers where equality, prefix search,
lexical range, or direct identifier projection matters. Examples include
callsigns, part numbers, customer IDs, account IDs, UUID-like strings, and
source-file identifiers. The 500-distinct-value mark is a good point to stop and
think, but it should not automatically force a switch from `StringEnum` to
`StringLexBSI`.

Use `StringSearch` for unstructured text where token search matters. For this
application, raw comments and long free-form descriptions belong there if we
decide they are queryable. Raw Cabrillo lines are better retained in source
files or object storage and represented in QS by parsed fields plus hashes.

## SWPC Historical Indices

The first SWPC target is a pair of compact tables:

- `swpc_daily_indices`
- `swpc_k_indices_3h`

`swpc_daily_indices` stores one row per UTC day with A/Ap and SFI/F10.7-style
values. `swpc_k_indices_3h` stores one row per UTC three-hour K/Kp bucket.

Use numeric keys, not date strings:

- `day_key`: `YYYYMMDD`, for example `20251130`
- `bucket_key`: `YYYYMMDDHH`, where `HH` is the UTC start hour of the three-hour
  bucket, for example `2025113021`

These are compact `IntBSI` primary keys and are also convenient relationship
targets. Spot and QSO payloads should carry:

- `spot_day_key`
- `spot_3h_bucket_key`
- `qso_day_key`
- `qso_3h_bucket_key`

That keeps time-window joins simple and avoids relying on runtime date
truncation for core application workflows.

Example analytical shape:

```sql
select
  s.band,
  s.dx_prefix,
  d.sfi,
  k.kp_index,
  count(*) as spots
from spots_flat s
inner join swpc_daily_indices d
  on s.spot_day_key = d.day_key
inner join swpc_k_indices_3h k
  on s.spot_3h_bucket_key = k.bucket_key
where s.spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by s.band, s.dx_prefix, d.sfi, k.kp_index
order by spots desc
limit 50;
```

The tables and spot payload keys are modeled. The SWPC ingester is still
implementation work.

## Cabrillo Tier 1 Scope

Contest logs can become very large, so the first Cabrillo ingest should be
regional and practical.

Tier 1 includes logs submitted by Caribbean and Central America stations. The
initial prefix seed list is:

```text
6Y 8P C6 CM CO FG FM FS HH HI HK0 HP HR J3 J6 J7 J8 KG4 KP2 KP4
P4 PJ2 PJ4 PJ5 PJ7 TG TI V2 V3 V4 VP2E VP2M VP2V VP5 VP9 XE YN YS ZF 9Y
```

The final inclusion decision should use the CTY/DXCC parser, not only string
prefix matching. Prefix matching is a download/indexing convenience; CTY
classification is the source of truth.

The first two tables are:

- `contest_logs`: one row per accepted submitted log
- `contest_qsos`: one row per parsed Cabrillo `QSO:` line

`contest_qsos.log_id` is a relationship vector to `contest_logs.log_id`.
`contest_qsos.qso_day_key` and `contest_qsos.qso_3h_bucket_key` are relationship
vectors to the SWPC tables. This makes propagation context directly joinable to
submitted contest QSOs.

## Cabrillo Parsing Rules

Own the parser in this repository. External libraries are useful references, but
the long-term value is in controlling the tolerant parsing, validation, and
normalization behavior.

Initial parser shape:

- read headers until the first `QSO:` line
- normalize category fields into separate `StringEnum` columns
- parse `QSO:` lines into timestamp, band/frequency, mode, worked callsign, and
  sent/received exchange fields
- derive station and worked-call prefix/continent with the CTY parser
- derive `qso_day_key` and `qso_3h_bucket_key`
- keep source-file metadata and a raw-line hash
- store raw Cabrillo source files outside QS so they can be reprocessed

Hard rejects should be limited to malformed QSO lines where the timestamp,
station, worked callsign, or frequency cannot be recovered. Header fields that
are missing or unusual should become `UNKNOWN` or `UNSPECIFIED`, not stop the
load.

## Join Targets

Useful first questions:

- Which Tier 1 stations were most spotted by RBN during CQWW?
- Which stations were heavily spotted but underrepresented in submitted logs?
- How did spot volume by band change across K/Kp buckets?
- Did high SFI periods shift Tier 1 activity toward higher bands?
- For a station, which RBN spotters heard them before, during, and after logged
  QSOs?

The important joins are by exact relationship where possible, and by bounded
time/frequency predicates where not:

- `contest_qsos.log_id -> contest_logs.log_id`
- `contest_qsos.qso_day_key -> swpc_daily_indices.day_key`
- `contest_qsos.qso_3h_bucket_key -> swpc_k_indices_3h.bucket_key`
- RBN spots to SWPC by precomputed day/3-hour keys once those fields are added
- RBN spots to Cabrillo QSOs by station/callsign, band, time window, and
  frequency proximity

## Source References

- Reverse Beacon Network archive: `https://reversebeacon.net/raw_data/`
- NOAA SWPC services: `https://services.swpc.noaa.gov/`
- CQ WW Cabrillo guidance: `https://cqww.com/cabrillo.htm`
