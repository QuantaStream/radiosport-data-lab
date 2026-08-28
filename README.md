# RadioSport Data Lab

RadioSport Data Lab is a small QuantaStream application playground for Reverse
Beacon Network spot data. It starts from the old `rbn-to-kinesis` idea but keeps
the new code plain: parse spots, normalize the event shape, enrich callsigns,
and feed QuantaStream through either SQL inserts or the streaming loader.

## Current Shape

- `configuration/` contains the first QuantaStream table descriptors.
- `internal/callsign/` contains first-party CTY/DXCC callsign parsing.
- `internal/rbn/` contains archive and telnet parsing primitives.
- `cmd/rbn-inspect` profiles an RBN daily archive and verifies the parser.
- `cmd/rbn-archive-to-jsonl` emits streaming-loader-ready JSONL from archives.
- `cmd/rbn-archive-load` POSTs archive batches to a running `qstream-loader`.
- `cmd/rbn-cache-build` builds focused parsed RBN day/callsign caches from
  archive files for repeatable contest matching.
- `cmd/rbn-telnet-sql-ingest` batches live telnet spots into prepared SQL inserts.
- `cmd/rbn-update-cty` refreshes local CTY/DXCC data for telnet enrichment.
- `cmd/rbn-qrz-lookup` fetches optional QRZ profiles and can cache them in SQL.
- `cmd/swpc-load` parses NOAA SWPC solar/geomagnetic indices and emits loader
  events for daily and three-hour propagation tables.
- `cmd/cabrillo-load` parses public Cabrillo contest logs and emits
  `contest_logs`, `activity_5m_buckets`, and `contest_qsos` loader events.
- `cmd/contest-spot-match-load` materializes QSO-to-RBN spot matches from a
  Cabrillo log and matching RBN archive files.
- `sql/views/` contains reusable analyst-facing views for QS SQL clients.
- `scripts/run-ti8x-contest-workflow.sh` runs the focused TI8X contest
  load/cache/match/view workflow against a running QS server and loader.
- `docs/SCHEMA_DESIGN.md` explains the mapper choices and ingestion plan.
- `docs/INGESTION_PLAN.md` defines the shared payload for SQL and streaming.
- `docs/ARCHIVE_PROFILE_20260821.md` records the 2026-08-21 sample profile.
- `docs/LOADER_BENCHMARK_20260823.md` records the local flat-loader baseline.
- `docs/CQWW_2025_STANDARD_BENCHMARK.md` records the two-day CQWW 2025
  standard-mode backfill and query pass.
- `docs/TI8X_BUCKET_RELOAD_20260828.md` records the focused TI8X reload with
  five-minute activity buckets.
- `docs/SWPC_AND_CONTEST_PLAN.md` defines the historical space-weather and
  Tier 1 Cabrillo contest-log expansion.
- `docs/QUERY_VIEWS.md` documents reusable views and smoke queries.
- `scripts/run-aws-distributed-flat-loader-benchmark.sh` repeats the AWS
  distributed flat-loader benchmark and captures a tarball of evidence.

## Data Sources

- RBN raw daily archives are zipped CSV files published by Reverse Beacon
  Network.
- The telnet stream emits live spot lines from `telnet.reversebeacon.net`.
- NOAA SWPC historical indices provide A, K/Kp, and SFI/F10.7 propagation
  context.
- Public adjudicated contest logs in Cabrillo format provide a submitted-log
  truth set. The first planned scope is Caribbean and Central America logs.
- QRZ XML lookups are optional enrichment and must be configured through
  environment variables. No QRZ credentials belong in this repository.

## Quick Check

```bash
go test ./...
go run ./cmd/rbn-inspect /tmp/rbn-data/20260821.zip
go run ./cmd/rbn-archive-to-jsonl /tmp/rbn-data/20260821.zip > /tmp/rbn-spots.jsonl
go run ./cmd/rbn-archive-load -target http://127.0.0.1:8088/ingest/json /tmp/rbn-data/20260821.zip
go run ./cmd/rbn-cache-build -cache-dir /tmp/rbn-cache-ti8x -dx-call TI8X /tmp/rbn-data/20251129.zip /tmp/rbn-data/20251130.zip
go run ./cmd/rbn-telnet-sql-ingest -dry-run -limit 10
go run ./cmd/rbn-update-cty
go run ./cmd/swpc-load -from 2026-08-21 -to 2026-08-21 > /tmp/swpc-20260821.jsonl
go run ./cmd/cabrillo-load -target "" https://cqww.com/publiclogs/2025cw/ti8x.log > /tmp/ti8x-contest.jsonl
go run ./cmd/contest-spot-match-load -target "" -rbn-cache /tmp/rbn-cache-ti8x https://cqww.com/publiclogs/2025cw/ti8x.log 2025-11-29 2025-11-30 > /tmp/ti8x-matches.jsonl
QRZ_USERNAME=... QRZ_PASSWORD=... go run ./cmd/rbn-qrz-lookup N7ZG
QRZ_USERNAME=... QRZ_PASSWORD=... go run ./cmd/rbn-telnet-sql-ingest -qrz-enrich
```

Relationship join smoke:

```sql
select
  s.spotted_at,
  s.dx_call,
  s.frequency_khz,
  q.country_name,
  q.grid,
  q.lookup_status
from spots s
inner join qrz_callsigns q
  on s.dx_call_ref = q.callsign
order by s.spotted_at desc
limit 50;
```

The archive parser skips the RBN footer line, validates callsign length, keeps
frequency in display-friendly kHz, and computes a stable synthetic `spot_id`.
Archive and live ingesters create pending QRZ parent rows before spots so
`spots.dx_call_ref` can exercise QS-native relationship joins; async QRZ
enrichment updates those rows later.

Archive and Cabrillo ingesters also compute five-minute activity buckets. The
shared `activity_5m_key` shape is `CALL|BAND|MODE|YYYYMMDDHHMM`, with the time
rounded down to the nearest UTC five-minute boundary. That gives analysts a
compact way to compare submitted contest QSOs with RBN spot density without
using runtime interval arithmetic in every query.

The shared `activity_5m_buckets` table is the parent dimension for this shape.
`spots_flat.activity_5m_ref` and `contest_qsos.activity_5m_ref` both point at
it, so QS can use native relationship-vector joins for bucket-level contest
analysis.

For loader pipeline tests, `spots_flat` provides the same spot fact shape without
the QRZ relationship vector. Use `rbn-archive-load -spot-type rbn_spot_flat
-qrz-parents=false -dense-spot-ids` to isolate raw archive ingestion throughput
with storage-friendly day-local column IDs. Pass multiple daily archive files
and `-day-workers N` to parallelize a historical backfill across days. Add
`-dx-call CALL` when you want a focused contest reload from very large archive
files.

SWPC backfills use the same loader endpoint. Start `qstream-loader` with
`activity_5m_buckets`, `spots_flat`, `contest_logs`, `contest_qsos`,
`swpc_daily_indices`, and `swpc_k_indices_3h` in its `-tables` allowlist when
running the full contest reload. Historical annual SWPC files are cached under
`data/swpc` when `-year` is used:

```bash
go run ./cmd/swpc-load \
  -year 2025 \
  -cache-dir data/swpc \
  -from 2025-11-29 \
  -to 2025-11-30 \
  -target http://127.0.0.1:8088/ingest/json \
  -parent-flush-wait 2s
```

Install the general RBN propagation view after the `spots_flat` and SWPC tables
exist:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/rbn_spot_propagation_base.sql
```

Install the submitted-log propagation view after the `contest_logs`,
`contest_qsos`, and SWPC tables exist:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_qso_propagation_base.sql
```

Install the joined spot/QSO five-minute activity view after both RBN spots and
Cabrillo QSOs are loaded:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_rbn_activity_5m_base.sql
```

Install the exact materialized match view after `contest_spot_matches` is
loaded:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_spot_match_base.sql
```

Load a single Cabrillo contest log through the JSON loader:

```bash
go run ./cmd/cabrillo-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -parent-flush-wait 2s \
  -cty-dat data/cty/cty.dat \
  https://cqww.com/publiclogs/2025cw/ti8x.log
```

The loader posts the `contest_logs` parent row first, waits briefly for the QS
loader to flush it, and then posts the `contest_qsos` child rows. The TI8X CQ WW
CW 2025 public log currently parses as one parent log and 3,947 QSOs.

```sql
select q.band, q.worked_continent, d.sfi, k.kp_index, count(*) as qsos
from contest_qsos q
inner join swpc_daily_indices d on q.qso_day_key = d.day_key
inner join swpc_k_indices_3h k on q.qso_3h_bucket_key = k.bucket_key
where q.station_call = 'TI8X'
group by q.band, q.worked_continent, d.sfi, k.kp_index
order by qsos desc
limit 30;
```

Five-minute bucket smoke:

```sql
select activity_5m_key, band, count(*) as rbn_spots
from spots_flat
group by activity_5m_key, band
order by rbn_spots desc
limit 20;
```

Spot/QSO bucket match smoke:

```sql
select activity_5m_key, activity_band, count(*) as qso_spot_pairs
from contest_rbn_activity_5m_base
where station_call = 'TI8X'
group by activity_5m_key, activity_band
order by qso_spot_pairs desc
limit 20;
```

Materialize exact QSO-to-spot matches for the focused TI8X reload:

```bash
./scripts/run-ti8x-contest-workflow.sh --reset
```

The script expects QuantaStream and `qstream-loader` to already be running with
the RadioSport schema directory. It creates the required tables, truncates the
workflow tables when `--reset` is passed, loads SWPC context, loads focused
`spots_flat` rows, builds the parsed RBN cache, materializes exact matches,
installs the views, and writes logs plus verification output under `/tmp`.

Manual form:

```bash
go run ./cmd/rbn-cache-build \
  -cache-dir /tmp/rbn-cache-ti8x \
  -dx-call TI8X \
  -dense-spot-ids \
  /tmp/rbn-data/20251129.zip \
  /tmp/rbn-data/20251130.zip

go run ./cmd/contest-spot-match-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -cty-dat data/cty/cty.dat \
  -rbn-cache /tmp/rbn-cache-ti8x \
  -dense-spot-ids \
  https://cqww.com/publiclogs/2025cw/ti8x.log \
  2025-11-29 \
  2025-11-30
```

The cache stores parsed spot JSONL by UTC day and DX callsign, so repeated
match-policy experiments do not rescan multi-million-row archive files. Build
the cache with the same focused `-dx-call` and dense-ID setting used for the
`spots_flat` load when `contest_spot_matches.spot_ref` should point back to
already loaded spot rows.

Exact match smoke:

```sql
select band, match_kind, count(*) as matches
from contest_spot_match_base
where station_call = 'TI8X'
group by band, match_kind
order by matches desc
limit 20;
```

Best-match smoke:

```sql
select band, count(*) as best_matches, avg(match_score) as avg_match_score
from contest_spot_match_base
where station_call = 'TI8X'
  and is_best_match = 1
group by band
order by best_matches desc
limit 20;
```

## Near-Term Build Plan

1. Benchmarks: compare SQL inserts and streaming loader throughput on the same
   daily archive.
2. Query examples: keep Workbench-friendly examples for live spots, QRZ
   enrichment, and relationship joins.
3. Additional feeds: add more radiosport data sources against the same schema
   style, starting with SWPC indices and Tier 1 Cabrillo contest logs.
