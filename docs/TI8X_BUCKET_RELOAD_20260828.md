# TI8X Five-Minute Bucket Reload

This note captures the local standard-mode reload used to validate five-minute
activity buckets for CQ WW CW 2025 TI8X analysis.

## Environment

- QuantaStream mode: standard single-node with native gRPC enabled
- MySQL endpoint: `127.0.0.1:4000`
- Native gRPC endpoint: `127.0.0.1:4100`
- Loader endpoint: `127.0.0.1:8088`
- Data directory: `/home/gmolinari/qstream-data/rbn-cqww2025-standard/data`
- Schema directory: `/home/gmolinari/projects/radiosport-data-lab/configuration`

## Loads

Repeat the full workflow against an already running local standard server and
`qstream-loader`:

```bash
./scripts/run-ti8x-contest-workflow.sh --reset
```

The script creates the required tables, truncates the workflow tables
child-first when `--reset` is passed, loads SWPC rows, loads focused TI8X
`spots_flat` rows, builds the parsed RBN cache, loads the Cabrillo log,
materializes exact matches from the cache, installs views, and stores logs plus
verification output under `/tmp/radiosport-ti8x-workflow-*`.

Use the explicit command sequence below when testing individual stages.

SWPC propagation context:

```bash
go run ./cmd/swpc-load \
  -year 2025 \
  -cache-dir data/swpc \
  -from 2025-11-29 \
  -to 2025-11-30 \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 100 \
  -parent-flush-wait 2s
```

Result: `daily_events=2`, `k_events=16`, `accepted=18`, `failed=0`.

Focused RBN archive load:

```bash
go run ./cmd/rbn-archive-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 2000 \
  -workers 4 \
  -day-workers 2 \
  -spot-type rbn_spot_flat \
  -qrz-parents=false \
  -dense-spot-ids \
  -dx-call TI8X \
  /tmp/rbn-data/20251129.zip \
  /tmp/rbn-data/20251130.zip
```

Result: `rows=12459223`, `emitted=5423`, `accepted=5423`, `failed=0`.
With activity parents enabled, this run emitted 418 `activity_5m_bucket` parent
events and 5,423 spot events, for 5,841 accepted loader events.

Contest log load:

```bash
go run ./cmd/cabrillo-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -parent-flush-wait 2s \
  -cty-dat data/cty/cty.dat \
  https://cqww.com/publiclogs/2025cw/ti8x.log
```

Result: `logs=1`, `qsos=3947`, `accepted=4445`, `failed=0`. The accepted count
includes one `contest_log` parent, 497 activity bucket parents, and 3,947
`contest_qso` rows.

## Verification

```sql
select count(*) as swpc_daily from swpc_daily_indices;
select count(*) as swpc_k_3h from swpc_k_indices_3h;
select count(*) as activity_buckets from activity_5m_buckets;
select count(*) as spots_flat_count, min(spotted_at), max(spotted_at)
from spots_flat;
select count(*) as contest_log_count from contest_logs;
select count(*) as contest_qso_count, min(qso_at), max(qso_at)
from contest_qsos;
```

Observed counts:

| Table | Rows |
| --- | ---: |
| `swpc_daily_indices` | 2 |
| `swpc_k_indices_3h` | 16 |
| `activity_5m_buckets` | 525 |
| `spots_flat` | 5,423 |
| `contest_logs` | 1 |
| `contest_qsos` | 3,947 |

Spot window: `2025-11-29 00:00:36` through `2025-11-30 23:59:57`.
QSO window: `2025-11-29 00:02:00` through `2025-11-30 23:58:00`.

Useful smoke query:

```sql
select
  activity_5m_key,
  band,
  worked_continent,
  sfi,
  kp_index,
  count(*) as qsos
from contest_qso_propagation_base
where station_call = 'TI8X'
group by activity_5m_key, band, worked_continent, sfi, kp_index
order by qsos desc
limit 20;
```

Native spot/QSO bucket match smoke:

```sql
select
  activity_5m_key,
  activity_band,
  count(*) as qso_spot_pairs
from contest_rbn_activity_5m_base
where station_call = 'TI8X'
group by activity_5m_key, activity_band
order by qso_spot_pairs desc
limit 20;
```

Observed leading buckets:

| Activity bucket | Band | QSO/spot pairs |
| --- | --- | ---: |
| `TI8X|10M|CW|202511301540` | `10M` | 600 |
| `TI8X|40M|CW|202511300145` | `40M` | 570 |
| `TI8X|10M|CW|202511291510` | `10M` | 560 |
| `TI8X|40M|CW|202511290145` | `40M` | 560 |
| `TI8X|10M|CW|202511301550` | `10M` | 477 |

Exact materialized match load:

```bash
go run ./cmd/rbn-cache-build \
  -cache-dir /tmp/rbn-cache-ti8x \
  -dx-call TI8X \
  -dense-spot-ids \
  /tmp/rbn-data/20251129.zip \
  /tmp/rbn-data/20251130.zip

go run ./cmd/contest-spot-match-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -cty-dat data/cty/cty.dat \
  -rbn-cache /tmp/rbn-cache-ti8x \
  -window 5m \
  https://cqww.com/publiclogs/2025cw/ti8x.log \
  2025-11-29 \
  2025-11-30
```

Result: `qsos=3947`, `spots=5423`, `matches=81248`, `accepted=81248`,
`failed=0`. Loader stats after the run showed `queued=0`, `records=81248`,
`flushes=36`, and `flush_errors=0`.

The first raw archive scan built `/tmp/rbn-cache-ti8x` in about 18 seconds after
reading 12,459,223 archive rows and caching the 5,423 TI8X spots. The
cache-backed match pass then produced the same 81,248 exact matches in about
1.8 seconds, which makes match-policy tuning practical without repeatedly
rescanning the large CQWW RBN files.

Exact match smoke:

```sql
select band, match_kind, count(*) as matches
from contest_spot_match_base
where station_call = 'TI8X'
group by band, match_kind
order by matches desc
limit 20;
```

Closest spotter sample:

```sql
select
  match_id,
  qso_id,
  spot_id,
  qso_at,
  spotted_at,
  time_delta_seconds,
  abs_frequency_delta_khz,
  match_rank,
  match_score,
  spotter_call,
  signal_db
from contest_spot_match_base
where station_call = 'TI8X'
  and abs_time_delta_seconds <= 60
order by match_score desc
limit 30;
```

Best match smoke:

```sql
select band, count(*) as best_matches, avg(match_score) as avg_match_score
from contest_spot_match_base
where station_call = 'TI8X'
  and is_best_match = 1
group by band
order by best_matches desc
limit 20;
```

## Product Notes

- `-dx-call` made focused two-day contest reloads quick while still reading the
  complete multi-million-row source archives.
- Parent/child relationship loads still need a first-class loader drain barrier.
  `-parent-flush-wait` works, but it is a userland convention.
- Joining `contest_qsos` directly to `spots_flat` on scalar `activity_5m_id` is
  still a non-relationship peer join and is rejected by QS. The implemented
  native path is `activity_5m_buckets`, exposed through
  `contest_rbn_activity_5m_base`.
- Exact spot-to-QSO matching is now represented by `contest_spot_matches`.
  Match rows carry deterministic proximity scores, a per-QSO rank, and an
  `is_best_match` flag while retaining the full candidate set for deeper review.
- Focused parsed RBN caches are the right middle layer for iterative contest
  analysis. They preserve the dense spot IDs used by focused archive loads and
  let match jobs rerun from day/callsign JSONL instead of scanning raw zips.
- `CREATE VIEW` stores view definitions under the data directory, but this run
  used an external schema directory at startup. The views had to be reinstalled
  after restart, so QS should clarify or improve view bootstrap when runtime
  config and stored catalog config are split.
