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
by hash.

