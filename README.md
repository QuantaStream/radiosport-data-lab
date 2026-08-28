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
- `cmd/rbn-telnet-sql-ingest` batches live telnet spots into prepared SQL inserts.
- `cmd/rbn-update-cty` refreshes local CTY/DXCC data for telnet enrichment.
- `cmd/rbn-qrz-lookup` fetches optional QRZ profiles and can cache them in SQL.
- `docs/SCHEMA_DESIGN.md` explains the mapper choices and ingestion plan.
- `docs/INGESTION_PLAN.md` defines the shared payload for SQL and streaming.
- `docs/ARCHIVE_PROFILE_20260821.md` records the 2026-08-21 sample profile.
- `docs/LOADER_BENCHMARK_20260823.md` records the local flat-loader baseline.
- `docs/CQWW_2025_STANDARD_BENCHMARK.md` records the two-day CQWW 2025
  standard-mode backfill and query pass.
- `docs/SWPC_AND_CONTEST_PLAN.md` defines the historical space-weather and
  Tier 1 Cabrillo contest-log expansion.
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
go run ./cmd/rbn-telnet-sql-ingest -dry-run -limit 10
go run ./cmd/rbn-update-cty
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

For loader pipeline tests, `spots_flat` provides the same spot fact shape without
the QRZ relationship vector. Use `rbn-archive-load -spot-type rbn_spot_flat
-qrz-parents=false -dense-spot-ids` to isolate raw archive ingestion throughput
with storage-friendly day-local column IDs. Pass multiple daily archive files
and `-day-workers N` to parallelize a historical backfill across days.

## Near-Term Build Plan

1. Benchmarks: compare SQL inserts and streaming loader throughput on the same
   daily archive.
2. Query examples: keep Workbench-friendly examples for live spots, QRZ
   enrichment, and relationship joins.
3. Additional feeds: add more radiosport data sources against the same schema
   style, starting with SWPC indices and Tier 1 Cabrillo contest logs.
