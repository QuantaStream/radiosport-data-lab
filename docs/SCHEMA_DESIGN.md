# Schema Design

## Source Shapes

The RBN archive for 2026-08-21 is a zipped CSV with this header:

```text
callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode
```

`callsign` is the spotting receiver. `dx` is the station being heard. The archive
already includes DXCC prefix and continent for both callsigns. The telnet feed is
lighter: it arrives as a text line and needs the callsign parser to add prefix,
continent, and band metadata.

## Tables

### `spots`

`spots` is the append-heavy fact table. It stores one row per received RBN spot.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `spot_id` | generated | `IntBSI` | Stable 63-bit hash, also `columnID`. |
| `spotted_at` | `date` or telnet UTC minute | `TimestampBSI` | Time quantum field. |
| `spotter_call` | `callsign` | `StringLexBSI length=16 maxLen=16` | Inline callsign, no KV remainder. |
| `dx_call` | `dx` | `StringLexBSI length=16 maxLen=16` | Inline callsign, no KV remainder. |
| `spotter_prefix`, `dx_prefix` | archive or parser | `StringEnum` | Low-cardinality DXCC prefix. |
| `spotter_continent`, `dx_continent` | archive or parser | `StringEnum` | Seven possible continent-style values. |
| `frequency_hz` | `freq * 1000` | `IntBSI` | Exact integer frequency for range predicates. |
| `band`, `mode`, `transmit_mode`, `source` | archive or parser | `StringEnum` | Small enumerations. |
| `signal_db`, `speed_wpm` | `db`, `speed` | `IntBSI` | Compact numeric ranges. |

The archive uses kHz as a floating number. The ingester normalizes to integer Hz
before insertion so queries can use exact integer comparisons.

### `qrz_callsigns`

`qrz_callsigns` is an optional enrichment table keyed by callsign. A spot can
exist without a QRZ profile. That makes this table suitable for outer joins,
cache warming, and background enrichment without blocking spot ingestion.

The callsign key uses `StringLexBSI length=16 maxLen=16`. Other string fields are
`StringEnum` until the data tells us a particular QRZ attribute has high enough
cardinality to deserve a different mapper.

## Ingestion Paths

### SQL Ingester

The SQL ingester should use prepared statements and batched inserts. It is the
best compatibility exercise because it goes through normal MySQL-facing client
paths.

### Streaming Loader Ingester

The streaming-loader ingester should emit the same normalized JSON shape used by
the SQL ingester. It is the path we expect to win for sustained archive backfill
or high-volume live stream ingestion.

## QRZ Enrichment

QRZ access should be optional and configured with environment variables such as
`QRZ_USERNAME` and `QRZ_PASSWORD` or a future secret provider. Missing QRZ rows
are normal and should be recorded as `lookup_status='not_found'` instead of
failing the spot pipeline.

Aliases should eventually be normalized into a small `qrz_aliases` table rather
than stored as one searchable comma-delimited string.
