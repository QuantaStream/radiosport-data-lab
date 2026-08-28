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

## Product Notes

- `-dx-call` made focused two-day contest reloads quick while still reading the
  complete multi-million-row source archives.
- Parent/child relationship loads still need a first-class loader drain barrier.
  `-parent-flush-wait` works, but it is a userland convention.
- Joining `contest_qsos` directly to `spots_flat` on scalar `activity_5m_id` is
  still a non-relationship peer join and is rejected by QS. The implemented
  native path is `activity_5m_buckets`, exposed through
  `contest_rbn_activity_5m_base`.
- Exact spot-to-QSO matching remains a future materialized-analysis feature when
  we want time-delta, frequency-delta, and confidence columns instead of coarse
  five-minute bucket correlation.
- `CREATE VIEW` stores view definitions under the data directory, but this run
  used an external schema directory at startup. The views had to be reinstalled
  after restart, so QS should clarify or improve view bootstrap when runtime
  config and stored catalog config are split.
