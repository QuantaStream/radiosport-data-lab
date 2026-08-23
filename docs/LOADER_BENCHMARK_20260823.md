# Local Loader Benchmark - 2026-08-23

Dataset: Reverse Beacon Network archive `20260821.zip`.

Rows accepted: 278,552 spot rows. The archive also contains one footer row,
which the parser skips.

## Finding

Archive backfills should use both:

- `quantastream-loader -physical-build-routing`
- `rbn-archive-load -dense-spot-ids`

This mirrors the TPC-H loader lesson: logical event identity and physical build
placement are different concerns. For this app, a stable FNV hash is useful as
an event identity, but it is a poor `columnID` value because it creates sparse,
high 64-bit row IDs. Dense archive IDs keep one day in a compact contiguous
column-id range and avoid pathological bitmap memory growth.

## Laptop Results

Command shape:

```bash
RBN_LOAD_LIMIT=0 RBN_LOAD_TIMEOUT=10m \
./scripts/run-local-flat-loader-matrix.sh /tmp/rbn-data/20260821.zip \
  1:1000 1:2000 4:1000
```

Results:

| Config | Accepted | Visible Rows | Seconds | Visible Rows/Sec |
| --- | ---: | ---: | ---: | ---: |
| `1:1000` | 278,552 | 278,552 | 11.637 | 23,936.75 |
| `1:2000` | 278,552 | 278,552 | 11.436 | 24,357.47 |
| `4:1000` | 278,552 | 278,552 | 13.361 | 20,848.14 |

For a single daily archive, extra POST workers do not help because
`spotted_at` routes all rows to the same physical day shard. Parallelism should
come from loading multiple independent day files rather than splitting one day
by hash. `rbn-archive-load -day-workers N` is the backfill path for that shape:
keep per-file `-workers` conservative, and increase `-day-workers` when loading
multiple daily archives.

## Four-Day Parallel Backfill

After adding `-day-workers`, a four-day local run loaded four independent daily
archives in parallel through one loader process:

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

Results:

| File | Rows | Accepted | Failed | Elapsed |
| --- | ---: | ---: | ---: | ---: |
| `20260818.zip` | 264,964 | 264,964 | 0 | 8.363s |
| `20260819.zip` | 322,861 | 322,861 | 0 | 9.451s |
| `20260820.zip` | 277,779 | 277,779 | 0 | 8.628s |
| `20260821.zip` | 278,552 | 278,552 | 0 | 8.675s |
| Total | 1,144,156 | 1,144,156 | 0 | 9.452s |

The database count matched the accepted total. The aggregate rate was roughly
121,049 rows/sec on the laptop. The similar per-file elapsed times are the
desired signal: each daily file routes to a separate physical day shard, so
parallelism comes from independent day-level build lanes rather than splitting
one day by logical hash.
