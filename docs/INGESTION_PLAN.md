# Ingestion Plan

Both ingesters should produce the same normalized spot record. Archive and
backfill files should use the batch/streaming loader path. Live telnet spots
should use prepared SQL inserts with sensible batching.

Archive and Cabrillo backfills also emit shared five-minute activity parent
rows before child facts. This keeps spot-to-QSO correlation on QS's native
relationship-vector path instead of requiring a peer-table join between facts.

## Normalized Spot Payload

```json
{
  "type": "rbn_spot",
  "data": {
    "spot_id": 123456789,
    "spotted_at": "2026-08-21T00:00:00Z",
    "spot_day_key": 20260821,
    "spot_3h_bucket_key": 2026082100,
    "spot_5m_bucket_key": 202608210000,
    "activity_5m_id": 3085122920447939943,
    "activity_5m_key": "KC2SIZ|20M|CW|202608210000",
    "spotter_call": "G4IRN",
    "spotter_prefix": "G",
    "spotter_continent": "EU",
    "dx_call": "KC2SIZ",
    "dx_prefix": "K",
    "dx_continent": "NA",
    "frequency_khz": 14054.4,
    "band": "20m",
    "mode": "CQ",
    "signal_db": 25,
    "speed_wpm": 13,
    "transmit_mode": "CW",
    "source": "archive"
  }
}
```

The SQL ingester can insert the inner `data` fields directly. The streaming
loader should emit the full object so `selector: type="rbn_spot"` can route the
event to the `spots` table. Loader-only flat backfills can instead emit
`type="rbn_spot_flat"` to target `spots_flat`, a relationship-free copy of the
spot fact shape.

`spots.dx_call_ref` is a relationship-vector field that reuses `/data/dx_call`
and points at `qrz_callsigns.callsign`. Ingesters must create a pending QRZ
parent row before inserting the first spot for a DX callsign.

`spots.activity_5m_ref`, `spots_flat.activity_5m_ref`, and
`contest_qsos.activity_5m_ref` reuse `/data/activity_5m_id` and point at
`activity_5m_buckets.activity_5m_id`. Archive, Cabrillo, and live SQL ingesters
ensure the activity parent exists before inserting child rows.

Current archive files include a full timestamp column. The parser also supports
older/minute-only shapes by deriving the date from `YYYYMMDD.csv` or
`YYYYMMDD.zip` file names.

## SQL Ingester

Use this path for arriving telnet data, not archive backfill. The live feed is
small enough for prepared inserts and a short flush window, and this gives us a
useful MySQL-compatible write workload.

Initial target:

```sql
insert into spots (
  spot_id,
  spotted_at,
  spot_day_key,
  spot_3h_bucket_key,
  spotter_call,
  spotter_prefix,
  spotter_continent,
  dx_call,
  dx_prefix,
  dx_continent,
  frequency_khz,
  band,
  mode,
  signal_db,
  speed_wpm,
  transmit_mode,
  source,
  spot_5m_bucket_key,
  activity_5m_id,
  activity_5m_key
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
```

Use prepared statements and client-side batches. The default live-stream policy
is to flush every 5 records or every 5 seconds, whichever comes first. That keeps
the telnet path near-real-time while still exercising the prepared batch path.

Initial dry-run:

```bash
go run ./cmd/rbn-telnet-sql-ingest \
  -dry-run \
  -limit 10
```

Initial SQL run:

```bash
go run ./cmd/rbn-telnet-sql-ingest \
  -mysql-dsn 'qstream@tcp(127.0.0.1:4000)/quanta' \
  -batch-size 5 \
  -batch-interval 5s
```

If `data/cty/cty.dat` exists, telnet ingestion enriches spotter and DX calls
with DXCC prefix and continent. Missing or unparseable callsigns fall back to
`UNKNOWN` prefix/continent and the spot is still ingested.

Before each SQL spot batch is committed, the ingester ensures a
`qrz_callsigns` row exists for every unique DX callsign in the batch. New rows
start with `lookup_status='pending'`. That keeps `spots.dx_call_ref` valid
without waiting on QRZ.

The SQL ingester also ensures one `activity_5m_buckets` row exists for every
unique activity bucket in the batch. Those rows are small and deterministic, so
they can be inserted synchronously without adding QRZ-style network latency.

Refresh CTY data explicitly:

```bash
go run ./cmd/rbn-update-cty \
  -dest data/cty/cty.dat
```

Optional QRZ cache enrichment:

```bash
QRZ_USERNAME=... QRZ_PASSWORD=... \
go run ./cmd/rbn-telnet-sql-ingest \
  -mysql-dsn 'qstream@tcp(127.0.0.1:4000)/quanta' \
  -cty-dat data/cty/cty.dat \
  -qrz-enrich \
  -qrz-queue-size 256 \
  -qrz-workers 1
```

QRZ network enrichment is deliberately async and lossy. `spots` are committed
first, and then callsigns are offered to a bounded queue with a non-blocking
send. If QRZ or the cache table falls behind, enrichment work is dropped and
counted instead of applying backpressure to the telnet stream. Successful QRZ
lookups update pending rows to `lookup_status='found'`; not-found lookups update
them to `lookup_status='not_found'`.

## Streaming Loader Ingester

Use this path for daily CSV archives and large historical backfills. SQL inserts
are useful for compatibility, but the loader should be the expected throughput
path for multi-year data.

Relationship-table target:

1. Parse archive records into the normalized payload.
2. Emit one `activity_5m_bucket` parent event before the first child spot for
   each bucket.
3. Emit one pending `qrz_callsign` event before the first spot for each DX call.
4. Write newline-delimited JSON records with `cmd/rbn-archive-to-jsonl`.
5. Or POST batches directly to the QuantaStream streaming loader with
   `cmd/rbn-archive-load`.

This path should be the sustained-throughput baseline.

Start `qstream-loader` with every target table in its allowlist. A full contest
reload normally includes `activity_5m_buckets`, `spots_flat`, `contest_logs`,
`contest_qsos`, `swpc_daily_indices`, and `swpc_k_indices_3h`.

Example direct loader run:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -activity-parents=true \
  /tmp/rbn-data/20260821.zip
```

Flat loader-throughput run:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -spot-type rbn_spot_flat \
  -qrz-parents=false \
  -activity-parents=true \
  -dense-spot-ids \
  /tmp/rbn-data/20260821.zip
```

Parallel flat backfill across multiple daily archive files:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 2000 \
  -workers 1 \
  -day-workers 4 \
  -spot-type rbn_spot_flat \
  -qrz-parents=false \
  -activity-parents=true \
  -dense-spot-ids \
  -dx-call TI8X \
  /tmp/rbn-data/20260818.zip \
  /tmp/rbn-data/20260819.zip \
  /tmp/rbn-data/20260820.zip \
  /tmp/rbn-data/20260821.zip
```

Use the flat path when the goal is measuring archive loader throughput without
relationship-vector or QRZ-cache work. Use the normal `rbn_spot` path when the
goal is exercising the full relationship-aware application model.

Archive backfills should use dense archive spot IDs when the target table marks
`spot_id` as `columnID: true`. The parser's stable hash remains useful for
event identity, but sparse hash values are a poor physical column-id shape for
bitmap storage. Dense archive IDs are allocated in day-local contiguous ranges
so one daily file builds compact storage artifacts and multiple days do not
collide.

For backfills, prefer parallelism across archive files with `-day-workers`
rather than increasing `-workers` inside one file. A single daily archive routes
to one physical day shard, so extra POST workers mostly add client-side pressure.
Multiple daily files give the loader independent physical build shards to drain.
The `-limit` flag applies per archive file. For throughput tests, choose daily
files with roughly similar row counts. One unusually large day will dominate the
wall-clock time even when smaller days finish quickly. For stress and sustained
load testing, include the large days deliberately and report the skew.

Use `-dx-call CALL` for focused contest studies. The loader still parses the
whole source archive, but only emits spots where the normalized DX call matches
the supplied callsign. That makes two-day contest reloads cheap enough for
iterative schema and query work.

Materialized spot/QSO matches are generated by `cmd/contest-spot-match-load`.
The command reads a Cabrillo log and one or more RBN archive files, filters RBN
spots to the submitted station call, then emits `contest_spot_match` rows for
spots inside the configured time window. By default it uses the same dense
archive spot ID policy as `rbn-archive-load -dense-spot-ids`, so `spot_ref`
points at the already loaded `spots_flat` rows.

Each emitted match keeps the full candidate row and adds deterministic ranking
metadata. `match_rank = 1` and `is_best_match = 1` identify the best spot for a
QSO. Ranking prefers closest time, then closest frequency, then strongest
signal, then lowest spot ID. The numeric `match_score` is a zero-to-100 blend
weighted primarily toward time proximity.

```bash
go run ./cmd/contest-spot-match-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -cty-dat data/cty/cty.dat \
  -window 5m \
  -frequency-tolerance-khz 0 \
  https://cqww.com/publiclogs/2025cw/ti8x.log \
  /tmp/rbn-data/20251129.zip \
  /tmp/rbn-data/20251130.zip
```

For repeated match experiments, build a focused parsed RBN cache once and point
the matcher at cache days instead of raw zip files:

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
  -window 5m \
  https://cqww.com/publiclogs/2025cw/ti8x.log \
  2025-11-29 \
  2025-11-30
```

The first cache slice is intentionally callsign-focused. Cache files live under
`<cache-dir>/YYYY/MM/DD/by_dx_call/<CALL>.jsonl`, with a day manifest alongside
them. Use the same focused callsign and dense-ID choice as the `spots_flat`
backfill when match rows need relationship-compatible `spot_ref` values.

Use `-frequency-tolerance-khz` when the match policy should reject spots that
are too far away from the logged QSO frequency. Use `-max-matches-per-qso` for a
nearest-N materialization when a full many-to-many window would be too dense.

Local flat-loader matrix:

```bash
RBN_LOAD_LIMIT=50000 \
./scripts/run-local-flat-loader-matrix.sh /tmp/rbn-data/20260821.zip \
  1:1000 2:1000 4:1000 8:1000
```

Each matrix entry starts a clean temporary QuantaStream-in-a-box server and a
loader pointed only at `spots_flat`, then records rows/sec in
`/tmp/radiosport-flat-loader-matrix.tsv`.

## QRZ Enrichment

QRZ lookups should be asynchronous relative to spot ingestion:

1. Extract unique callsigns from `spotter_call` and `dx_call`.
2. Check the local `qrz_callsigns` cache.
3. Fetch missing profiles from QRZ when credentials are configured.
4. Ensure a pending profile row exists when the callsign is first referenced.
5. Update successful profiles with `lookup_status='found'`.
6. Update not-found/cache-negative records with `lookup_status='not_found'`.

Environment variables:

```bash
QRZ_USERNAME=...
QRZ_PASSWORD=...
```

Lookup-only smoke:

```bash
QRZ_USERNAME=... QRZ_PASSWORD=... \
go run ./cmd/rbn-qrz-lookup N7ZG K1ABC
```

Cache profiles into QuantaStream:

```bash
QRZ_USERNAME=... QRZ_PASSWORD=... \
go run ./cmd/rbn-qrz-lookup \
  -insert \
  -mysql-dsn 'qstream@tcp(127.0.0.1:4000)/quanta' \
  -cty-dat data/cty/cty.dat \
  N7ZG K1ABC
```

The spot pipeline must continue when QRZ is unreachable or when a callsign is not
listed. The command writes `lookup_status='found'` for successful lookups and
`lookup_status='not_found'` for cache-negative rows. QRZ credentials are runtime
configuration only; never commit them.

## SWPC Space-Weather Backfill

Historical A, K/Kp, and SFI/F10.7 values should be loaded as independent,
joinable time-series facts before contest analysis.

`cmd/swpc-load` reads NOAA SWPC daily solar and geomagnetic text products,
merges rows by UTC day, and emits one `swpc_daily_index` event plus up to eight
`swpc_k_index_3h` events per day. Without `-target`, it writes JSONL to stdout
for inspection. With `-target`, it posts batches to a running `qstream-loader`.

Recent SWPC smoke:

```bash
go run ./cmd/swpc-load \
  -from 2026-08-21 \
  -to 2026-08-21 \
  > /tmp/swpc-20260821.jsonl
```

Loader smoke:

```bash
go run ./cmd/swpc-load \
  -from 2026-08-21 \
  -to 2026-08-21 \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 20
```

For historical contest windows, use `-year` to load the annual SWPC daily solar
and daily geomagnetic files. The command reuses cached files from `data/swpc`
when present and downloads `YYYY_DSD.txt` plus `YYYY_DGD.txt` when missing. The
default historical source list tries the SWPC archive path first and then a
NOAA report mirror; use `-historical-base-url` to override or narrow that list.

```bash
go run ./cmd/swpc-load \
  -year 2025 \
  -cache-dir data/swpc \
  -from 2025-11-29 \
  -to 2025-11-30 \
  -target http://127.0.0.1:8088/ingest/json
```

If remote historical fetches are unavailable, stage the annual files in
`data/swpc` with the expected names, or pass them explicitly:

```bash
go run ./cmd/swpc-load \
  -solar-source /path/to/2025_DSD.txt \
  -geomag-source /path/to/2025_DGD.txt \
  -from 2025-11-29 \
  -to 2025-11-30 \
  -target http://127.0.0.1:8088/ingest/json
```

Initial loader payloads:

```json
{
  "type": "swpc_daily_index",
  "data": {
    "day_key": 20251130,
    "observed_date": "2025-11-30T00:00:00Z",
    "a_index": 8,
    "ap_index": 8,
    "sfi": 185.4,
    "sunspot_number": 94,
    "source": "swpc",
    "loaded_at": "2026-08-28T00:00:00Z"
  }
}
```

```json
{
  "type": "swpc_k_index_3h",
  "data": {
    "bucket_key": 2025113021,
    "bucket_start": "2025-11-30T21:00:00Z",
    "day_key": 20251130,
    "k_index": 2,
    "kp_index": 2.33,
    "source": "swpc",
    "loaded_at": "2026-08-28T00:00:00Z"
  }
}
```

Spot payloads include `spot_day_key` and `spot_3h_bucket_key`. The `spots_flat`
archive schema keeps those values as scalar keys and also maps them into
`spot_day_ref` and `spot_3h_bucket_ref` relationship-vector fields for joins to
SWPC daily and three-hour rows. Contest QSO payloads already reserve
`qso_day_key` and `qso_3h_bucket_key` in the schema plan. These precomputed
keys make joins deterministic and avoid runtime date bucketing as a prerequisite
for normal analysis.

## Cabrillo Contest Log Backfill

The first Cabrillo ingest scope is Tier 1 Caribbean and Central America contest
logs. Full worldwide contest logs, and especially full US competitive-landscape
analysis, should be a later scale milestone.

Initial Tier 1 prefix seed list:

```text
6Y 8P C6 CM CO FG FM FS HH HI HK0 HP HR J3 J6 J7 J8 KG4 KP2 KP4
P4 PJ2 PJ4 PJ5 PJ7 TG TI V2 V3 V4 VP2E VP2M VP2V VP5 VP9 XE YN YS ZF 9Y
```

Use this list for rough filtering and discovery, but use the CTY parser for the
final inclusion decision.

Initial Cabrillo flow:

1. Download or stage public adjudicated Cabrillo logs for one contest.
2. Parse headers and `QSO:` lines with first-party code.
3. Classify submitted station and worked callsigns with CTY/DXCC data.
4. Keep only Tier 1 submitted-station logs for the first pass.
5. Emit one `contest_log` event per accepted log.
6. Emit one `activity_5m_bucket` parent event per unique QSO activity bucket.
7. Emit one `contest_qso` event per parsed QSO.
8. Store raw Cabrillo source files outside QS for reprocessing and audit.

Hard rejects should be limited to unrecoverable QSO structure. Unknown or
unusual headers should become explicit `UNKNOWN` or `UNSPECIFIED` enum values so
the loader can keep moving.

The useful first joins are:

- `contest_qsos.log_id -> contest_logs.log_id`
- `contest_qsos.qso_day_key -> swpc_daily_indices.day_key`
- `contest_qsos.qso_3h_bucket_key -> swpc_k_indices_3h.bucket_key`
- `contest_qsos.activity_5m_ref -> activity_5m_buckets.activity_5m_id`
- `spots_flat.activity_5m_ref -> activity_5m_buckets.activity_5m_id`
- RBN spots to SWPC through `spot_day_ref` and `spot_3h_bucket_ref`
- RBN spots to Cabrillo QSOs by shared activity bucket, or by exact bounded
  time/frequency proximity through scored `contest_spot_matches`
