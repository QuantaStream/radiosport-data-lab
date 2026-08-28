# CQWW 2025 Standard-Mode Backfill

This note captures a laptop standard-mode QuantaStream run against two Reverse
Beacon Network archive files from the 2025 CQWW CW contest weekend.

Source archives:

- `20251129.zip`
- `20251130.zip`

The files came from the Reverse Beacon Network daily archive area:
`https://data.reversebeacon.net/rbn_history/`.

## Environment

- QuantaStream mode: standard single-node with native gRPC enabled
- MySQL endpoint: `127.0.0.1:4000`
- Native gRPC endpoint: `127.0.0.1:4100`
- Loader endpoint: `127.0.0.1:8088`
- Loader mode: `standard-native`
- Loader workers: `12`
- Target table: `spots_flat`
- Callsign mapper: `StringLexBSI length=8 maxLen=16`
- Spot IDs: dense archive IDs
- Physical build routing: enabled

The `spots_flat` table intentionally avoids the QRZ relationship vector so this
run isolates raw archive backfill and queryability.

## Load Command

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 2000 \
  -workers 1 \
  -day-workers 2 \
  -spot-type rbn_spot_flat \
  -qrz-parents=false \
  -dense-spot-ids \
  /tmp/rbn-data/20251129.zip \
  /tmp/rbn-data/20251130.zip
```

## Load Results

| File | Rows | Accepted | Failed | Rejected | Elapsed |
| --- | ---: | ---: | ---: | ---: | ---: |
| `20251129.zip` | 6,006,150 | 6,006,150 | 0 | 0 | 7m48.042s |
| `20251130.zip` | 6,453,073 | 6,453,073 | 0 | 0 | 8m39.198s |
| Total | 12,459,223 | 12,459,223 | 0 | 0 | 8m39.200s |

Final loader stats showed zero queued records, zero open sessions, and zero
flush errors. The visible SQL count matched the accepted total:

```sql
select count(*) as spots_flat_count from spots_flat;
-- 12459223
```

The laptop sustained roughly 24,000 accepted archive rows/sec for this two-day
contest load. The client elapsed time is dominated by the larger of the two day
files, which is the expected behavior for day-shard parallelism.

## Query Timings

Representative MySQL-compatible queries against the loaded 12.46M-row table:

| Query Shape | Result | Time |
| --- | ---: | ---: |
| `count(*)` | 12,459,223 | 0.011s |
| `group by band, mode` Top 20 | 20 rows | 0.320s |
| `group by dx_prefix` Top 25 | 25 rows | 0.452s |
| `group by spotter_continent, dx_continent` | 36 rows | 0.297s |
| `band = '20m'`, group by prefix, average signal | 20 rows | 0.200s |
| `spotted_at >= todate('2025-11-30')`, group by prefix/band | 30 rows | 1.228s |
| `dx_call LIKE 'N7%'` count | 31,249 | 0.242s |
| `dx_call LIKE 'N7%'` detail/order/limit | 20 rows | 0.478s |
| `dx_continent = 'NA' and band = '10m'`, group by prefix | 20 rows | 0.164s |

The strongest signal is unchanged from the AWS runs: categorical analytics over
`StringEnum`, `TimestampBSI`, and numeric mappers are fast even immediately
after large backfills.

## Contest Examples

Top DX stations by spot count:

| DX Call | Spots |
| --- | ---: |
| `9A1A` | 85,655 |
| `ES9C` | 85,304 |
| `UA7K` | 81,943 |
| `TK0C` | 77,222 |
| `DF0HQ` | 74,937 |

Top spotting receivers:

| Spotter | Spots |
| --- | ---: |
| `DO4DXA` | 211,581 |
| `DF2CK` | 209,894 |
| `DL9GTB` | 169,688 |
| `G4IRN` | 155,125 |
| `LZ4AE` | 153,790 |

Top band/mode pairs:

| Band | Mode | Spots |
| --- | --- | ---: |
| `40m` | `CQ` | 3,273,017 |
| `20m` | `CQ` | 2,605,641 |
| `15m` | `CQ` | 2,437,099 |
| `80m` | `CQ` | 2,038,921 |
| `10m` | `CQ` | 1,463,518 |

## Callsign Length Distribution

The CQWW data reinforces the eight-byte callsign prefix decision:

| Length | Distinct Calls | Rows |
| ---: | ---: | ---: |
| 3 | 877 | 296,024 |
| 4 | 18,156 | 7,524,355 |
| 5 | 21,989 | 3,118,809 |
| 6 | 14,468 | 1,433,422 |
| 7 | 958 | 20,911 |
| 8 | 813 | 34,167 |
| 9 | 184 | 30,380 |
| 10 | 39 | 1,026 |
| 11 | 4 | 42 |
| 12 | 2 | 87 |

Rows with eight or fewer callsign characters account for roughly 99.75% of this
contest dataset. Longer callsigns are uncommon but real, so `maxLen=16` remains
important for correctness while the eight-byte BSI prefix keeps the common path
compact.

## Optimization Finding

Full callsign grouped Top-N is correct but still slow compared with categorical
analytics:

| Query Shape | Time |
| --- | ---: |
| `group by dx_call order by count(*) desc limit 20` | 87.296s |
| `group by spotter_call order by count(*) desc limit 20` | 1m43.91s |
| `group by char_length(dx_call)` | 109.013s |

This is the same class of finding captured for AWS testing: `StringLexBSI` is
excellent for equality, prefix filtering, selective projection, and residual
long-string validation, while grouped aggregates over full high-cardinality
strings want either a native grouped-string path or a derived summary table.

## Candidate Adjacent Feeds

Good next data sources to evaluate:

- NOAA SWPC JSON products for Kp, solar flux, and other propagation context.
- PSK Reporter for digital-mode reception reports and another high-volume
  near-real-time stream.
- WSPR/WSPR.live for beacon-style propagation records and very large historical
  analytical data.
- POTA/SOTA spots and activations as joinable activity/event dimensions.
- DX Cluster feeds as the human-spotted counterpart to RBN's skimmer-generated
  stream.
