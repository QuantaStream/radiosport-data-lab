# Schema Design

## Source Shapes

The RBN archive for 2026-08-21 is a zipped CSV with this header:

```text
callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode
```

`callsign` is the spotting receiver. `dx` is the station being heard. The current
archive shape already includes a full timestamp plus DXCC prefix and continent
for both callsigns. Older/minute-only archive shapes can derive the day from the
file name. The telnet feed is lighter: it arrives as a text line and needs the
callsign parser to add prefix, continent, and band metadata.

## Tables

### `spots`

`spots` is the append-heavy fact table. It stores one row per received RBN spot.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `spot_id` | generated | `IntBSI` | Stable 63-bit hash, also `columnID`. |
| `spotted_at` | `date` or telnet UTC minute | `TimestampBSI` | Time quantum field. |
| `spotter_call` | `callsign` | `StringLexBSI length=16 maxLen=16` | Inline callsign, no KV remainder. |
| `dx_call` | `dx` | `StringLexBSI length=16 maxLen=16` | Inline callsign, no KV remainder. |
| `dx_call_ref` | `dx` | `ParentRelation -> qrz_callsigns` | Relationship vector for QS-native joins to QRZ enrichment rows. |
| `spotter_prefix`, `dx_prefix` | archive or parser | `StringEnum` | Low-cardinality DXCC prefix. |
| `spotter_continent`, `dx_continent` | archive or parser | `StringEnum` | Seven possible continent-style values. |
| `frequency_khz` | `freq` | `FloatScaleBSI`, `scale: 1` | Human-readable RBN frequency with fixed one-decimal precision for range predicates. |
| `band`, `mode`, `transmit_mode`, `source` | archive or parser | `StringEnum` | Small enumerations. |
| `signal_db`, `speed_wpm` | `db`, `speed` | `IntBSI` | Compact numeric ranges. |

The archive uses kHz as a floating number. The ingester stores kHz with one
decimal place so Workbench and ad hoc radio queries display the familiar value.

### `qrz_callsigns`

`qrz_callsigns` is an optional enrichment table keyed by callsign. A spot can
exist without a final QRZ profile, but supported ingesters create a lightweight
`pending` parent row before inserting the spot. That makes the table suitable for
relationship joins, cache warming, and background enrichment without blocking
spot ingestion.

The callsign key uses `StringLexBSI length=16 maxLen=16`. Other string fields are
`StringEnum` until the data tells us a particular QRZ attribute has high enough
cardinality to deserve a different mapper.

`spots.dx_call_ref` points at `qrz_callsigns.callsign`. Live SQL ingestion writes
a `lookup_status='pending'` QRZ stub before inserting a spot for a new DX call.
The async QRZ worker later updates that row to `found` or `not_found`. Archive
JSON/loader paths emit the same pending parent row before the first spot for each
DX callsign, keeping the relationship valid for backfill.

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

`cmd/rbn-qrz-lookup` performs direct QRZ XML lookups. With `-insert`, it writes
the mapped row into `qrz_callsigns`; without `-insert`, it prints JSON so the
shape can be inspected. CTY enrichment can fill DXCC prefix, continent, country,
CQ zone, and ITU zone when QRZ omits those values.

Live telnet ingestion creates pending QRZ parent rows in-band so the
`spots.dx_call_ref` relationship can be maintained. Optional QRZ cache warming
with `-qrz-enrich` still runs behind a bounded async queue after spot commits, so
QRZ network latency never slows the telnet read/insert path.

Aliases should eventually be normalized into a small `qrz_aliases` table rather
than stored as one searchable comma-delimited string.

## Enrichment Failure Rule

CTY/DXCC enrichment is local file data. The app looks for `RBN_CTY_DAT` first,
then `data/cty/cty.dat`. Use `cmd/rbn-update-cty` to refresh the file
explicitly; normal ingestion does not download country data at startup.

Spot ingestion is lossless-first. If the core spot structure is valid but DXCC or
QRZ enrichment fails, the spot row is still inserted. Prefix and continent fields
fall back to `UNKNOWN`, and enrichment failures should be counted/logged for
parser improvement work.

Hard rejects are limited to malformed spot structure, invalid core callsign
shape, unparseable frequency, unparseable signal/speed, or unusable timestamp.
