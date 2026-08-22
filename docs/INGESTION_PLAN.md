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
    "spotter_call": "G4IRN",
    "spotter_prefix": "G",
    "spotter_continent": "EU",
    "dx_call": "KC2SIZ",
    "dx_prefix": "K",
    "dx_continent": "NA",
    "frequency_hz": 14054400,
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
event to the `spots` table.

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
  spotter_call,
  spotter_prefix,
  spotter_continent,
  dx_call,
  dx_prefix,
  dx_continent,
  frequency_hz,
  band,
  mode,
  signal_db,
  speed_wpm,
  transmit_mode,
  source
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
  -mysql-dsn 'MOLIG004@tcp(127.0.0.1:4000)/quanta' \
  -batch-size 5 \
  -batch-interval 5s
```

If `data/cty/cty.dat` exists, telnet ingestion enriches spotter and DX calls
with DXCC prefix and continent. Missing or unparseable callsigns fall back to
`UNKNOWN` prefix/continent and the spot is still ingested.

Refresh CTY data explicitly:

```bash
go run ./cmd/rbn-update-cty \
  -dest data/cty/cty.dat
```

Optional QRZ cache enrichment:

```bash
QRZ_USERNAME=... QRZ_PASSWORD=... \
go run ./cmd/rbn-telnet-sql-ingest \
  -mysql-dsn 'MOLIG004@tcp(127.0.0.1:4000)/quanta' \
  -cty-dat data/cty/cty.dat \
  -qrz-enrich \
  -qrz-queue-size 256 \
  -qrz-workers 1
```

QRZ enrichment is deliberately async and lossy. `spots` are committed first, and
then callsigns are offered to a bounded queue with a non-blocking send. If QRZ or
the cache table falls behind, enrichment work is dropped and counted instead of
applying backpressure to the telnet stream.

## Streaming Loader Ingester

Use this path for daily CSV archives and large historical backfills. SQL inserts
are useful for compatibility, but the loader should be the expected throughput
path for multi-year data.

Initial target:

1. Parse archive or telnet records into the normalized payload.
2. Write newline-delimited JSON records with `cmd/rbn-archive-to-jsonl`.
3. Or POST batches directly to the QuantaStream streaming loader with
   `cmd/rbn-archive-load`.

This path should be the sustained-throughput baseline.

Example direct loader run:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  /tmp/rbn-data/20260821.zip
```

## QRZ Enrichment

QRZ lookups should be asynchronous relative to spot ingestion:

1. Extract unique callsigns from `spotter_call` and `dx_call`.
2. Check the local `qrz_callsigns` cache.
3. Fetch missing profiles from QRZ when credentials are configured.
4. Insert successful profiles with `lookup_status='found'`.
5. Insert not-found/cache-negative records with `lookup_status='not_found'`.

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
  -mysql-dsn 'MOLIG004@tcp(127.0.0.1:4000)/quanta' \
  -cty-dat data/cty/cty.dat \
  N7ZG K1ABC
```

The spot pipeline must continue when QRZ is unreachable or when a callsign is not
listed. The command writes `lookup_status='found'` for successful lookups and
`lookup_status='not_found'` for cache-negative rows. QRZ credentials are runtime
configuration only; never commit them.
