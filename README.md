# RadioSport Data Lab

RadioSport Data Lab is a small QuantaStream application playground for Reverse
Beacon Network spot data. It starts from the old `rbn-to-kinesis` idea but keeps
the new code plain: parse spots, normalize the event shape, enrich callsigns,
and feed QuantaStream through either SQL inserts or the streaming loader.

## Current Shape

- `configuration/` contains the first QuantaStream table descriptors.
- `internal/rbn/` contains archive and telnet parsing primitives.
- `cmd/rbn-inspect` profiles an RBN daily archive and verifies the parser.
- `docs/SCHEMA_DESIGN.md` explains the mapper choices and ingestion plan.
- `docs/INGESTION_PLAN.md` defines the shared payload for SQL and streaming.
- `docs/ARCHIVE_PROFILE_20260821.md` records the 2026-08-21 sample profile.

## Data Sources

- RBN raw daily archives are zipped CSV files published by Reverse Beacon
  Network.
- The telnet stream emits live spot lines from `telnet.reversebeacon.net`.
- QRZ XML lookups are optional enrichment and must be configured through
  environment variables. No QRZ credentials belong in this repository.

## Quick Check

```bash
go test ./...
go run ./cmd/rbn-inspect /tmp/rbn-data/20260821.zip
```

The archive parser skips the RBN footer line, validates callsign length, converts
frequency from kHz to integer Hz, and computes a stable synthetic `spot_id`.

## Near-Term Build Plan

1. SQL ingester: prepared/batched inserts into the `spots` and `qrz_callsigns`
   tables.
2. Streaming loader ingester: emit the same normalized JSON event shape through
   QuantaStream's streaming loader.
3. Callsign enrichment: plug in the CTY/DXCC parser for telnet spots and use
   QRZ as an optional profile cache.
4. Benchmarks: compare SQL inserts and streaming loader throughput on the same
   daily archive.
