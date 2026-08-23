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

## AWS Distributed Backfill

AWS testing used a three-node distributed QuantaStream cluster with the
`spots_flat` table, `StringLexBSI length=8 maxLen=16` callsign fields, dense
archive spot IDs, physical build routing, and a loader process with 24 shard
workers.

Loader shape:

```bash
go run ./cmd/quantastream-loader \
  -connection-mode distributed \
  -consul-addr 127.0.0.1:8500 \
  -config-dir /home/ubuntu/radiosport-data-lab/configuration \
  -listen 127.0.0.1:8088 \
  -tables spots_flat \
  -workers 24 \
  -channel-size 1000000 \
  -physical-build-routing
```

Balanced twelve-day archive load:

```bash
./scripts/run-aws-distributed-flat-loader-benchmark.sh
```

The script defaults to the same twelve balanced archive files used for this
benchmark, truncates `spots_flat`, runs the archive loader, waits for loader
drain and visible row count parity, captures loader stats and representative
query timings, and writes a timestamped tarball under `/tmp`. It records both
client accepted rows/sec and visible rows/sec after drain. Pass explicit archive
paths to test a different file set.

Results:

| Run | Days | Rows | Accepted | Failed | Elapsed | Rows/Sec |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Balanced day set | 12 | 3,702,151 | 3,702,151 | 0 | 12.425s | 297,960 |

The database count matched exactly:

```sql
select count(*) as spots_flat_count from spots_flat;
-- 3702151
```

Loader stats after the balanced run showed no queue backlog, no open sessions,
and zero flush errors.

An intentionally larger sixteen-day run loaded 8,224,848 rows in 70.666s
(~116,390 rows/sec). That run mixed several much larger weekend files with
normal daily files, so the slowest large day dominated elapsed time. It is still
a useful sustained-load sanity check, but the balanced twelve-day set is the
cleaner throughput measurement for day-shard parallelism.

## AWS Queryability After Load

The loaded data was immediately queryable through the MySQL-compatible proxy.
Representative query timings on the balanced 3.7M-row `spots_flat` dataset:

| Query Shape | Result | Time |
| --- | ---: | ---: |
| `dx_call LIKE 'N7%'` count | 19,047 | 0.155s |
| `dx_call LIKE 'OE/DL7UZO%'` count | 350 | 0.137s |
| `dx_call LIKE 'OE/DL7UZO%'` detail/order/limit | 20 rows | 0.04s |
| `group by band, mode` | 20 rows | 0.39s |
| `group by dx_prefix` Top 25 | 25 rows | 0.81s |
| `group by spotter_continent, dx_continent` | 36 rows | 0.25s |
| `band = '20m'`, group by prefix, average signal | 20 rows | 0.07s |
| `spotted_at >= todate('2026-08-20')`, group by prefix/band | 30 rows | 0.04s |

These are the strongest workload signals from the test:

- `StringLexBSI length=8 maxLen=16` is a good callsign fit. The common path stays
  compact, and uncommon longer callsigns remain correct through backing-string
  rehydration.
- Hybrid prefix filtering is essential. A long-prefix lookup such as
  `OE/DL7UZO%` uses the first eight bytes as a native BSI candidate range, then
  residual-checks the full callsign string.
- Backing-string materialization is fast when the bitmap candidate set is
  selective. The projected long-callsign detail query returned in roughly 40ms.
- Low/medium-cardinality `StringEnum` grouping is already very fast.

Two optimization findings were captured as post-release GitHub issues:

- High-cardinality grouped Top-N over full `StringLexBSI` callsigns is correct
  but slower than the categorical aggregate shapes.
  See `QuantaStream/quantastream#13`.
- `topn()` and `IS NOT NULL` filtered grouped Top-N queries are correct but can
  miss the fastest grouped aggregate path.
  See `QuantaStream/quantastream#14`.

## Callsign Length Distribution

The measured `dx_call` lengths on the 3.7M-row balanced AWS dataset strongly
support an eight-byte prefix:

| Length | Distinct Calls | Rows |
| ---: | ---: | ---: |
| 3 | 441 | 14,896 |
| 4 | 9,105 | 818,289 |
| 5 | 14,603 | 1,415,799 |
| 6 | 12,097 | 1,187,780 |
| 7 | 1,356 | 130,599 |
| 8 | 1,414 | 107,197 |
| 9 | 268 | 15,454 |
| 10 | 165 | 8,885 |
| 11 | 43 | 3,016 |
| 12 | 11 | 236 |

Rows with eight or fewer characters account for roughly 99.25% of the measured
dataset. Longer callsigns are uncommon but real, so `maxLen=16` remains useful
for correctness and projection.
