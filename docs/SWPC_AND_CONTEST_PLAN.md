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
- `spot_5m_bucket_key`
- `qso_day_key`
- `qso_3h_bucket_key`
- `qso_5m_bucket_key`
- `activity_5m_id`
- `activity_5m_ref`
- `activity_5m_key`

That keeps time-window joins simple and avoids relying on runtime date
truncation for core application workflows.

The shared five-minute activity key shape is
`CALL|BAND|MODE|YYYYMMDDHHMM`, where the call is the spotted DX station for RBN
spots and the submitted station for Cabrillo QSOs. The hash-backed
`activity_5m_id` is the compact comparison key; the string form is retained for
Workbench/Tableau readability and ad hoc debugging.

`activity_5m_buckets` is now the shared parent table for those bucket keys.
`spots_flat.activity_5m_ref` and `contest_qsos.activity_5m_ref` both reference
it, which turns spot-to-QSO bucket matching into a native relationship-vector
join.

`cmd/swpc-load` implements the first backfill path. It reads SWPC daily solar
and daily geomagnetic text products, merges them by UTC day, and emits loader
events for both tables. Recent SWPC products are available from the default
service URLs; older contest windows can be loaded from annual `YYYY_DSD.txt`
and `YYYY_DGD.txt` files cached under `data/swpc`. The default historical
source list tries the SWPC archive path first and then a NOAA report mirror:

```bash
go run ./cmd/swpc-load \
  -year 2025 \
  -cache-dir data/swpc \
  -from 2025-11-29 \
  -to 2025-11-30 \
  -target http://127.0.0.1:8088/ingest/json \
  -parent-flush-wait 2s
```

If remote historical fetches are unavailable, place `2025_DSD.txt` and
`2025_DGD.txt` in `data/swpc`, or pass file paths with `-solar-source` and
`-geomag-source`.

When posting through `qstream-loader`, include both SWPC tables in the loader
allowlist. Full contest reloads should also include `activity_5m_buckets`,
`spots_flat`, `contest_logs`, and `contest_qsos`. The loader commits its native
session on graceful shutdown, so long-running backfills should either keep the
loader up for more batches or stop it cleanly before doing a cold readback
verification.

Create the reusable spot/SWPC query view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/rbn_spot_propagation_base.sql
```

Example analytical shape through that view:

```sql
select
  band,
  dx_prefix,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by band, dx_prefix, sfi, kp_index
order by spots desc
limit 50;
```

The tables, spot payload keys, and SWPC ingester are implemented. Historical
SWPC reloads for contest windows such as CQ WW CW 2025 are now covered by
`-year`, `-cache-dir`, `-from`, and `-to`.

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

`cmd/cabrillo-load` implements the first Cabrillo path. It can read a local file
or public URL, emit JSONL for inspection, or post directly to `qstream-loader`.
It sends the parent `contest_logs` row before `contest_qsos` and waits briefly
for the loader to flush the parent so relationship-vector validation succeeds.

```bash
go run ./cmd/cabrillo-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -cty-dat data/cty/cty.dat \
  https://cqww.com/publiclogs/2025cw/ti8x.log
```

The TI8X CQ WW CW 2025 public log is a useful first smoke: one `contest_logs`
row, 3,947 `contest_qsos` rows, and zero parser rejects with the current public
source file.

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
- derive `qso_5m_bucket_key`, `activity_5m_id`, and `activity_5m_key`
- emit `activity_5m_bucket` parent rows before `contest_qso` child rows
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
- RBN spots to Cabrillo QSOs by station/callsign, band, five-minute activity
  bucket, and eventually frequency proximity

Spot-to-QSO five-minute matching should use `activity_5m_buckets` or the
`contest_rbn_activity_5m_base` view. Directly joining `spots_flat` to
`contest_qsos` on scalar `activity_5m_id` remains a non-relationship peer-table
join and is intentionally not the preferred QS modeling path.

## Product Improvements Seen During Reload

- Loader parent/child backfills need an explicit flush or drain barrier. The
  current ingesters handle this with a brief parent phase wait, but a first-class
  loader feature would make relationship-vector loads less hand-tuned.
- Selective archive backfills are valuable. `-dx-call` keeps focused contest
  reloads fast enough for iterative analysis even when the source files contain
  millions of unrelated RBN spots.
- Bucketed correlation now has a first-class modeling pattern. Shared activity
  buckets turn useful peer-table comparisons into native relationship-vector
  joins.
- Exact spot-to-QSO matching will eventually want a materialized match table
  that records time delta, frequency delta, and confidence. The shared bucket is
  the fast coarse match layer.

## Source References

- Reverse Beacon Network archive: `https://reversebeacon.net/raw_data/`
- NOAA SWPC services: `https://services.swpc.noaa.gov/`
- CQ WW Cabrillo guidance: `https://cqww.com/cabrillo.htm`
