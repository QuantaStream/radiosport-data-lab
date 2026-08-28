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

SWPC space-weather data is modeled as small historical index tables keyed by UTC
day and UTC three-hour bucket. The key values are numeric `YYYYMMDD` and
`YYYYMMDDHH` integers so spots and contest QSOs can join to propagation context
without runtime date truncation.

Cabrillo contest logs are modeled in two pieces: one submitted log row and many
parsed `QSO:` rows. The first ingest scope is Tier 1 Caribbean and Central
America stations, with final inclusion based on CTY/DXCC parsing.

## Mapper Guidance

Cardinality alone should not decide string mapping. Around 500 distinct values
is a useful prompt to think, but it is not an automatic `StringLexBSI` threshold.

Use `StringEnum` for categorical values, including fields that may grow into the
low thousands, when the field is commonly used for `GROUP BY`, `topn()`,
dashboards, equality filters, or dimensional joins. Bands, modes, continents,
DXCC prefixes, countries, categories, and statuses all fit this shape.

Use `StringLexBSI` for bounded identifiers. Callsigns, part numbers, customer
IDs, account IDs, UUID-like values, and source-file identifiers are the natural
fit. These fields benefit from equality, prefix search, lexical range, and
direct projection without treating every identifier as a small category.

Use `StringSearch` for unstructured text. Raw comments or long descriptions can
be searchable if the application needs that behavior. Raw Cabrillo lines should
stay in source files or object storage and be represented in QS by parsed fields
plus hashes.

## Tables

### `spots`

`spots` is the append-heavy fact table. It stores one row per received RBN spot.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `spot_id` | generated | `IntBSI` | Stable 63-bit hash, also `columnID`. |
| `spotted_at` | `date` or telnet UTC minute | `TimestampBSI` | Time quantum field. |
| `spot_day_key` | derived `YYYYMMDD` | `IntBSI` | UTC day key for SWPC bucketing and day-level filters. |
| `spot_3h_bucket_key` | derived `YYYYMMDDHH` | `IntBSI` | UTC three-hour bucket key for K/Kp bucketing and display. |
| `spot_5m_bucket_key` | derived `YYYYMMDDHHMM` | `IntBSI` | UTC five-minute bucket key for contest spot/QSO correlation. |
| `activity_5m_id` | derived hash | `IntBSI` | Compact hash of `DX_CALL|BAND|MODE|five-minute bucket`. |
| `activity_5m_ref` | derived hash | `ParentRelation -> activity_5m_buckets` | Relationship vector for native spot/QSO bucket joins. |
| `activity_5m_key` | derived string | `StringLexBSI length=32 maxLen=64` | Debuggable bucket key for Workbench/Tableau queries. |
| `spotter_call` | `callsign` | `StringLexBSI length=8 maxLen=16` | Eight-byte lexical prefix keeps common callsigns in a compact BSI width; uncommon longer callsigns use backing-string rehydration for full projection. |
| `dx_call` | `dx` | `StringLexBSI length=8 maxLen=16` | Prefix searches such as `LIKE 'N7%'` remain native BSI range predicates while avoiding the wider 16-byte BSI payload. |
| `dx_call_ref` | `dx` | `ParentRelation -> qrz_callsigns` | Relationship vector for QS-native joins to QRZ enrichment rows. |
| `spotter_prefix`, `dx_prefix` | archive or parser | `StringEnum` | Low-cardinality DXCC prefix. |
| `spotter_continent`, `dx_continent` | archive or parser | `StringEnum` | Seven possible continent-style values. |
| `frequency_khz` | `freq` | `FloatScaleBSI`, `scale: 1` | Human-readable RBN frequency with fixed one-decimal precision for range predicates. |
| `band`, `mode`, `transmit_mode`, `source` | archive or parser | `StringEnum` | Small enumerations. |
| `signal_db`, `speed_wpm` | `db`, `speed` | `IntBSI` | Compact numeric ranges. |

The archive uses kHz as a floating number. The ingester stores kHz with one
decimal place so Workbench and ad hoc radio queries display the familiar value.

### `spots_flat`

`spots_flat` mirrors the spot fact columns but intentionally omits
`dx_call_ref`. It exists as a loader-throughput comparison table for archive
backfills where we want to measure raw spot ingestion without QRZ parent stubs
or QRZ reverse relationship artifacts in the path. It also maps
`spot_day_key` into `spot_day_ref` and `spot_3h_bucket_key` into
`spot_3h_bucket_ref`, giving contest archives QS-native relationship joins to
the tiny SWPC parent tables.

`spots_flat` also carries `spot_5m_bucket_key`, `activity_5m_id`,
`activity_5m_ref`, and `activity_5m_key` for comparing spotted activity against
submitted contest QSO activity through the shared bucket parent.

The table uses selector `type="rbn_spot_flat"`. Archive tools can target it with
`-spot-type rbn_spot_flat -qrz-parents=false -dense-spot-ids`. The dense-id
option is important for loader backfills because `spot_id` is the QS column ID:
day-local contiguous IDs build compact bitmap artifacts, while stable hash IDs
are intentionally sparse and are better treated as event identity.

### `activity_5m_buckets`

`activity_5m_buckets` is a shared parent dimension for bucket-level contest
analysis. Both RBN spots and Cabrillo QSOs derive the same activity key when
they refer to the same station, band, mode, and UTC five-minute bucket. Modeling
that key as a parent table lets QS use relationship-vector joins instead of a
peer-table equality join between two fact tables.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `activity_5m_id` | derived hash | `IntBSI` | Primary key and column ID. |
| `activity_5m_key` | derived string | `StringLexBSI length=32 maxLen=64` | Human-readable `CALL|BAND|MODE|YYYYMMDDHHMM` key. |
| `activity_call` | derived call | `StringLexBSI length=8 maxLen=16` | Normalized station/callsign for the shared activity bucket. |
| `activity_band` | derived band | `StringEnum` | Normalized band. |
| `activity_mode` | derived mode | `StringEnum` | Normalized mode. |
| `bucket_key` | derived `YYYYMMDDHHMM` | `IntBSI` | UTC five-minute bucket key. |
| `bucket_start` | derived timestamp | `TimestampBSI` | UTC bucket start timestamp. |

### `qrz_callsigns`

`qrz_callsigns` is an optional enrichment table keyed by callsign. A spot can
exist without a final QRZ profile, but supported ingesters create a lightweight
`pending` parent row before inserting the spot. That makes the table suitable for
relationship joins, cache warming, and background enrichment without blocking
spot ingestion.

The callsign key uses `StringLexBSI length=8 maxLen=16` for the same compact
prefix-BSI tradeoff as the spot callsign fields. Other string fields are
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

### `swpc_daily_indices`

`swpc_daily_indices` stores one row per UTC day with historical A/Ap and
SFI/F10.7-style values.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `day_key` | generated `YYYYMMDD` | `IntBSI` | Primary key and column ID. |
| `observed_date` | SWPC product date | `TimestampBSI` | UTC day timestamp. |
| `a_index`, `ap_index` | SWPC daily indices | `IntBSI` | Daily geomagnetic activity values. |
| `sfi` | SWPC solar flux value | `FloatScaleBSI`, `scale: 1` | F10.7/SFI-style value. |
| `sunspot_number` | SWPC product value | `IntBSI` | Optional but useful solar context. |
| `source` | ingester | `StringEnum` | Source product label. |
| `loaded_at` | ingester | `TimestampBSI` | Load timestamp. |

### `swpc_k_indices_3h`

`swpc_k_indices_3h` stores one row per UTC three-hour K/Kp bucket. Its
`day_key` is a relationship vector to `swpc_daily_indices`.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `bucket_key` | generated `YYYYMMDDHH` | `IntBSI` | Primary key and column ID. |
| `bucket_start` | SWPC product timestamp | `TimestampBSI` | UTC bucket start. |
| `day_key` | generated `YYYYMMDD` | `ParentRelation` | Joins to `swpc_daily_indices`. |
| `k_index` | SWPC product value | `IntBSI` | Integer K where available. |
| `kp_index` | SWPC product value | `FloatScaleBSI`, `scale: 2` | Planetary Kp-style value. |
| `source` | ingester | `StringEnum` | Source product label. |
| `loaded_at` | ingester | `TimestampBSI` | Load timestamp. |

Spot payloads carry `spot_day_key` and `spot_3h_bucket_key` for scalar filters.
The `spots_flat` archive table maps those same payload values into
`spot_day_ref` and `spot_3h_bucket_ref` relationship vectors for QS-native
joins. Load SWPC parent rows before spot archives when running
propagation-aware backfills. The live `spots` table keeps scalar SWPC keys until
a live SWPC poller/stubber guarantees parent rows before telnet inserts.

### `contest_logs`

`contest_logs` stores one accepted Cabrillo submission. The first ingest scope
is Tier 1 Caribbean and Central America logs, not all worldwide logs.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `log_id` | generated | `StringLexBSI length=16 maxLen=96` | Stable log identity. |
| `contest_id` | source | `StringEnum` | Example: `cqww-cw-2025`. |
| `station_call` | Cabrillo header | `StringLexBSI length=8 maxLen=16` | Submitted station. |
| `station_prefix`, `station_continent`, `station_country` | CTY parser | `StringEnum` | Regional classification. |
| `cq_zone`, `itu_zone` | header or CTY parser | `IntBSI` | Contest geography. |
| `category_*` | Cabrillo headers | `StringEnum` | Normalized category dimensions. |
| `claimed_score`, `qso_count` | header/parser | `IntBSI` | Claimed result and parsed size. |
| `scope_region` | ingester | `StringEnum` | `tier1_caribbean_central_america`. |
| `source_file` | ingester | `StringLexBSI length=16 maxLen=160` | Source artifact identity. |
| `loaded_at` | ingester | `TimestampBSI` | Load timestamp. |

### `contest_qsos`

`contest_qsos` stores one parsed Cabrillo `QSO:` line. It links to both the
submitted log and SWPC time buckets.

| Column | Source | Mapper | Notes |
| --- | --- | --- | --- |
| `qso_id` | generated | `IntBSI` | Primary key and column ID. |
| `log_id` | generated | `ParentRelation` | Joins to `contest_logs`. |
| `contest_id` | source | `StringEnum` | Keeps common filters local. |
| `qso_at` | QSO line | `TimestampBSI` | UTC QSO time. |
| `qso_day_key` | generated `YYYYMMDD` | `ParentRelation` | Joins to `swpc_daily_indices`. |
| `qso_3h_bucket_key` | generated `YYYYMMDDHH` | `ParentRelation` | Joins to `swpc_k_indices_3h`. |
| `qso_5m_bucket_key` | generated `YYYYMMDDHHMM` | `IntBSI` | UTC five-minute bucket for matching RBN activity. |
| `activity_5m_id` | derived hash | `IntBSI` | Compact hash of `STATION_CALL|BAND|MODE|five-minute bucket`. |
| `activity_5m_ref` | derived hash | `ParentRelation -> activity_5m_buckets` | Relationship vector for native joins to RBN spot buckets. |
| `activity_5m_key` | derived string | `StringLexBSI length=32 maxLen=64` | Debuggable bucket key shared with RBN spot activity. |
| `station_call`, `worked_call` | QSO line | `StringLexBSI length=8 maxLen=16` | Identifier fields. |
| `station_prefix`, `station_continent`, `worked_prefix`, `worked_continent` | CTY parser | `StringEnum` | Contest geography. |
| `frequency_khz` | QSO line | `FloatScaleBSI`, `scale: 1` | Comparable with RBN spot frequency. |
| `band`, `mode` | derived/QSO line | `StringEnum` | Core contest dimensions. |
| `sent_exchange`, `received_exchange` | QSO line | `StringEnum` | Contest exchange values. |
| `source_file` | ingester | `StringLexBSI length=16 maxLen=160` | Source artifact identity. |

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
