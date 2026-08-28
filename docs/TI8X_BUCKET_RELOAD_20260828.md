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

Contest log load:

```bash
go run ./cmd/cabrillo-load \
  -target http://127.0.0.1:8088/ingest/json \
  -batch-size 1000 \
  -parent-flush-wait 2s \
  -cty-dat data/cty/cty.dat \
  https://cqww.com/publiclogs/2025cw/ti8x.log
```

Result: `logs=1`, `qsos=3947`, `accepted=3948`, `failed=0`.

## Verification

```sql
select count(*) as swpc_daily from swpc_daily_indices;
select count(*) as swpc_k_3h from swpc_k_indices_3h;
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

## Product Notes

- `-dx-call` made focused two-day contest reloads quick while still reading the
  complete multi-million-row source archives.
- Parent/child relationship loads still need a first-class loader drain barrier.
  `-parent-flush-wait` works, but it is a userland convention.
- Joining `contest_qsos` directly to `spots_flat` on `activity_5m_id` is still a
  non-relationship peer join and is rejected by QS. The likely product path is a
  shared `activity_5m_buckets` parent table or a materialized QSO-to-spot match
  table for contest analysis.
- `CREATE VIEW` stores view definitions under the data directory, but this run
  used an external schema directory at startup. The views had to be reinstalled
  after restart, so QS should clarify or improve view bootstrap when runtime
  config and stored catalog config are split.
