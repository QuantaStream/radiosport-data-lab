# Ingestion Plan

Both ingesters should produce the same normalized spot record. Archive and
backfill files should use the batch/streaming loader path. Live telnet spots
should use prepared SQL inserts with sensible batching.

## Normalized Spot Payload

```json
{
  "type": "rbn_spot",
  "data": {
    "spot_id": 123456789,
    "spotted_at": "2026-08-21T00:00:00Z",
    "spot_day_key": 20260821,
    "spot_3h_bucket_key": 2026082100,
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
  source
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
2. Emit one pending `qrz_callsign` event before the first spot for each DX call.
3. Write newline-delimited JSON records with `cmd/rbn-archive-to-jsonl`.
4. Or POST batches directly to the QuantaStream streaming loader with
   `cmd/rbn-archive-load`.

This path should be the sustained-throughput baseline.

Example direct loader run:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  /tmp/rbn-data/20260821.zip
```

Flat loader-throughput run:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -spot-type rbn_spot_flat \
  -qrz-parents=false \
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
  -dense-spot-ids \
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

Spot payloads include `spot_day_key` and `spot_3h_bucket_key`. Contest QSO
payloads already reserve `qso_day_key` and `qso_3h_bucket_key` in the schema
plan. These precomputed keys make joins deterministic and avoid runtime date
bucketing as a prerequisite for normal analysis.

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
6. Emit one `contest_qso` event per parsed QSO.
7. Store raw Cabrillo source files outside QS for reprocessing and audit.

Hard rejects should be limited to unrecoverable QSO structure. Unknown or
unusual headers should become explicit `UNKNOWN` or `UNSPECIFIED` enum values so
the loader can keep moving.

The useful first joins are:

- `contest_qsos.log_id -> contest_logs.log_id`
- `contest_qsos.qso_day_key -> swpc_daily_indices.day_key`
- `contest_qsos.qso_3h_bucket_key -> swpc_k_indices_3h.bucket_key`
- RBN spots to SWPC by spot day/bucket keys after spot payloads are extended
- RBN spots to Cabrillo QSOs by callsign, band, bounded time window, and
  frequency proximity
