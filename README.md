# RadioSport Data Lab

RadioSport Data Lab is a contest intelligence toolkit for amateur radio
operators. It loads public contest logs, Reverse Beacon Network archives, SWPC
space-weather data, and station metadata into QuantaStream so contesters can
explain what happened, find missed opportunity, and plan the next event.

The goal is not to require a BI product or an AI subscription. The toolkit
should be useful from a terminal with repeatable commands and reports. Codex can
then sit on top as an optional analysis partner: it can help choose workflows,
compare stations, interpret query output, and turn the data into a practical
contest plan.

## North Star

Load your logs, public competitor logs, RBN archives, and solar data. Then use
QuantaStream to understand where propagation, station capability, operator
choices, and multiplier strategy shaped the result.

The central questions are:

- Where did I lose rate versus comparable stations?
- Which openings did competitors exploit that I missed?
- Was a weak band propagation-limited, antenna-limited, or underused?
- Which multipliers were available but missed?
- How broadly was my signal heard after spotter bias is calibrated?
- What should my hour-by-hour plan look like for the next contest?

## Usage Modes

- **Manual mode:** clone the repo, run the loaders, install the views, and
  generate reports from saved SQL/query packs.
- **Guided mode:** use canned workflows and documented reports for common
  contests, stations, competitors, and propagation questions.
- **Codex mode:** ask richer questions in plain language and let Codex assemble
  the workflow, inspect the results, and suggest next queries.

Visualization is intentionally optional. Tableau, MySQL Workbench, CSV exports,
and future static HTML charts are all output paths. The core product is the
repeatable data pipeline and analysis model.

## Recommended Workstation

RadioSport Data Lab is written in Go and works best on a developer workstation
with fast local storage and enough memory to load multi-day RBN archives. The
reference setup is a modern laptop or desktop running Windows with WSL2/Ubuntu,
Go installed inside Ubuntu, and QuantaStream running from the same Linux
environment.

A practical starting point is:

- modern multi-core CPU
- 32 GB RAM or more
- SSD/NVMe storage with plenty of free space for archives and QS data
- Windows 11 with WSL2/Ubuntu, or native Linux
- Go installed when building from source

Smaller machines can still run focused examples and single-day experiments.
Large contest backfills, competitor comparisons, and RBN propagation studies
benefit directly from more CPU, RAM, and disk bandwidth. On a laptop, prefer
focused station filters and two-to-four-day RBN windows. Treat broad multi-week
RBN sweeps as an AWS or offline-cache job until the loader and storage profile
are hardened further.

## License

RadioSport Data Lab is released under the MIT License. QuantaStream itself is a
separate project with its own license; this repository does not relicense
QuantaStream.

The repository should contain code, schemas, docs, SQL, and workflow scripts.
Downloaded contest logs, RBN archives, QRZ-derived data, QuantaStream data
directories, and generated reports belong in local runtime directories and
should not be committed.

NOAA/SWPC space-weather files are small public U.S. government datasets and may
be committed as a convenience cache for reproducible examples. They can be
refreshed with `cmd/swpc-load`.

## Data Model

The toolkit combines four evidence streams:

- **Contest logs:** the submitted-log truth set for your station and comparable
  competitors.
- **RBN spots:** after-the-fact propagation evidence and signal observations.
- **SWPC indices:** SFI/F10.7, A, K, and Kp context for contest and analog
  propagation windows.
- **Station metadata:** callsign/DXCC/continent/zone enrichment plus optional
  spotter calibration profiles.

QuantaStream is used because these workloads are naturally bitmap-shaped:
time buckets, bands, prefixes, zones, continents, callsigns, and relationship
joins over large event streams.

## Repository Map

Core schema and parsing:

- `configuration/` contains the QuantaStream table descriptors.
- `internal/callsign/` contains first-party CTY/DXCC callsign parsing.
- `internal/rbn/` contains archive and telnet parsing primitives.

Loaders and builders:

- `cmd/rbn-inspect` profiles an RBN daily archive and verifies the parser.
- `cmd/rbn-archive-scan` counts selected DX calls across staged RBN archives
  before loading, which helps pick laptop-sized propagation windows.
- `cmd/rbn-archive-to-jsonl` emits streaming-loader-ready JSONL from archives.
- `cmd/rbn-archive-load` POSTs archive batches to a running `qstream-loader`.
- `cmd/rbn-cache-build` builds focused parsed RBN day/callsign caches from
  archive files for repeatable contest matching.
- `cmd/rbn-telnet-sql-ingest` batches live telnet spots into prepared SQL
  inserts.
- `cmd/rbn-update-cty` refreshes local CTY/DXCC data for telnet enrichment.
- `cmd/rbn-qrz-lookup` fetches optional QRZ profiles and can cache them in SQL.
- `cmd/swpc-load` parses NOAA SWPC solar/geomagnetic indices and emits loader
  events for daily and three-hour propagation tables.
- `cmd/cabrillo-load` parses public Cabrillo contest logs and emits
  `contest_logs`, `activity_5m_buckets`, and `contest_qsos` loader events.
- `cmd/contest-spot-match-load` materializes QSO-to-RBN spot matches from a
  Cabrillo log and matching RBN archive files, with optional spotter-profile
  calibration fields for normalized signal analysis.
- `cmd/spotter-profile-build` computes current RBN spotter calibration profiles
  from loaded spot data and emits loader events, including CTY-derived country
  centroid geo fields when `cty.dat` is available.
- `cmd/station-activity-build` materializes five-minute station activity
  summaries for missed-opening and cohort analysis.

Queries, views, and workflows:

- `sql/views/` contains reusable analyst-facing views for QS SQL clients.
- `sql/queries/contest_competitiveness_smoke.sql` contains repeatable
  competitiveness smoke queries for MySQL CLI, Workbench, and visualization
  checks.
- `scripts/run-ti8x-contest-workflow.sh` runs the focused TI8X contest
  load/cache/match/view workflow, with optional competitor packs, against a
  running QS server and loader.
- `scripts/run-aws-distributed-flat-loader-benchmark.sh` repeats the AWS
  distributed flat-loader benchmark and captures a tarball of evidence.

Documentation:

- `docs/SCHEMA_DESIGN.md` explains the mapper choices and ingestion plan.
- `docs/INGESTION_PLAN.md` defines the shared payload for SQL and streaming.
- `docs/RUNNING_LAB.md` is the local restart, health-check, and workflow
  runbook.
- `docs/QUERY_SAMPLER.md` contains copy/paste SQL for Workbench, Tableau, and
  CLI exploration.
- `docs/QUERY_VIEWS.md` documents reusable views and smoke queries.
- `docs/SWPC_AND_CONTEST_PLAN.md` defines the historical space-weather and
  Tier 1 Cabrillo contest-log expansion.
- `docs/CQWW_SSB_2026_PLANNING.md` captures the first CQWW SSB 2026 planning
  parameters: peer logs, SFI analog years, RBN propagation-window rules, and
  report outputs.
- `docs/SPOTTER_PROFILES_DESIGN.md` sketches spotter calibration, weighted SNR,
  and reach-scoring tables/views.
- `docs/MISSED_OPENINGS_DESIGN.md` sketches station activity summaries and
  missed-opening analysis.
- `docs/CONTEST_COMPETITIVENESS_ANALYSIS.md` explains the first
  competitiveness views and worksheets.
- `docs/ARCHIVE_PROFILE_20260821.md` records the 2026-08-21 sample profile.
- `docs/LOADER_BENCHMARK_20260823.md` records the local flat-loader baseline.
- `docs/CQWW_2025_STANDARD_BENCHMARK.md` records the two-day CQWW 2025
  standard-mode backfill and query pass.
- `docs/TI8X_BUCKET_RELOAD_20260828.md` records the focused TI8X reload with
  five-minute activity buckets.

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
go run ./cmd/rbn-archive-scan -dx-calls TI8X,8P5A,V47T /tmp/rbn-data/2025*.zip
go run ./cmd/rbn-archive-to-jsonl /tmp/rbn-data/20260821.zip > /tmp/rbn-spots.jsonl
go run ./cmd/rbn-archive-load -target http://127.0.0.1:8088/ingest/json /tmp/rbn-data/20260821.zip
go run ./cmd/rbn-cache-build -cache-dir /tmp/rbn-cache-ti8x -dx-call TI8X /tmp/rbn-data/20251129.zip /tmp/rbn-data/20251130.zip
go run ./cmd/rbn-telnet-sql-ingest -dry-run -limit 10
go run ./cmd/rbn-update-cty
go run ./cmd/swpc-load -from 2026-08-21 -to 2026-08-21 > /tmp/swpc-20260821.jsonl
go run ./cmd/cabrillo-load -target "" https://cqww.com/publiclogs/2025cw/ti8x.log > /tmp/ti8x-contest.jsonl
go run ./cmd/contest-spot-match-load -target "" -rbn-cache /tmp/rbn-cache-ti8x -spotter-profile-mysql-dsn 'qstream@tcp(127.0.0.1:4000)/quanta?parseTime=true' https://cqww.com/publiclogs/2025cw/ti8x.log 2025-11-29 2025-11-30 > /tmp/ti8x-matches.jsonl
go run ./cmd/spotter-profile-build -target "" -from 2025-11-29 -to 2025-12-01 -cty-dat data/cty/cty.dat > /tmp/spotter-profiles.jsonl
go run ./cmd/station-activity-build -target "" -from 2025-11-29 -to 2025-12-01 -dx-call TI8X > /tmp/station-activity.jsonl
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
`contest_spot_matches`, `rbn_spotter_nodes`, `spotter_profile_snapshots`,
`spotter_profiles`, `station_activity_5m_summaries`, `swpc_daily_indices`, and
`swpc_k_indices_3h` in its `-tables` allowlist when running the full contest
reload. Historical annual SWPC files are cached under `data/swpc` when `-year`
is used:

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
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_best_spot_match_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_calibrated_spot_match_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_competitiveness_qso_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_competitiveness_signal_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/spotter_profile_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/station_activity_5m_base.sql
```

The match views expose Tableau-friendly UTC date parts such as `qso_hour`,
`qso_day_of_week`, and `spotted_hour`, so common hour-by-band worksheets can
use plain columns instead of generated date-part SQL.

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
`spots_flat` rows, builds the parsed RBN cache, loads one or more Cabrillo logs,
materializes exact matches for each log, builds station activity summaries,
installs the views, and writes logs plus verification output under `/tmp`.
Spotter profile calibration is opt-in with `RBN_PROFILE_BUILD=1` while its
relationship key path is being hardened.

Schema activation and reset go through the MySQL-compatible endpoint with
`CREATE TABLE <name>` and `TRUNCATE TABLE <name>`. That keeps single-node and
distributed runs aligned: start `quantastream` with `-config-dir
./configuration`, or start `quantastream-proxy` with `-schema-dir
./configuration`.

Small competitor-pack reload:

```bash
CONTEST_STATIONS=TI8X,V47T,8P5A \
CONTEST_LOG_URLS=https://cqww.com/publiclogs/2025cw/ti8x.log,https://cqww.com/publiclogs/2025cw/v47t.log,https://cqww.com/publiclogs/2025cw/8p5a.log \
./scripts/run-ti8x-contest-workflow.sh --reset
```

The workflow uses the same callsign watchlist for the filtered RBN spot load and
the parsed RBN cache so dense archive spot IDs remain aligned. This is a DX-call
filter only: all RBN spotter/receiver stations that heard those DX calls remain
in `spots_flat`, including important Caribbean spotters such as `TI7W`. Larger
Caribbean comparison packs can add public Cabrillo logs such as `PJ4K` and
`ZF1A`.

Manual form:

```bash
go run ./cmd/rbn-cache-build \
  -cache-dir /tmp/rbn-cache-ti8x \
  -dx-call TI8X,V47T,8P5A \
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
select qso_hour, band, count(*) as best_matches, avg(match_score) as avg_match_score
from contest_best_spot_match_base
where station_call = 'TI8X'
group by qso_hour, band
order by qso_hour, band
limit 20;
```

## Near-Term Build Plan

1. Benchmarks: compare SQL inserts and streaming loader throughput on the same
   daily archive.
2. Query examples: keep Workbench-friendly examples for live spots, QRZ
   enrichment, and relationship joins.
3. Additional feeds: add more radiosport data sources against the same schema
   style, starting with SWPC indices and Tier 1 Cabrillo contest logs.
