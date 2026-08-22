# Ingestion Plan

Both ingesters should produce the same normalized spot record. The only
difference is the transport into QuantaStream.

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

## SQL Ingester

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

Use prepared statements and client-side batches. This path exercises
MySQL-compatible writes and is the right correctness baseline.

## Streaming Loader Ingester

Initial target:

1. Parse archive or telnet records into the normalized payload.
2. Write newline-delimited JSON records.
3. Feed those records to the QuantaStream streaming loader.

This path should be the sustained-throughput baseline once SQL correctness is
solid.

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

The spot pipeline must continue when QRZ is unreachable or when a callsign is not
listed.
