# Query Views

This repository keeps reusable analyst-facing views under `sql/views`.

## `rbn_spot_propagation_base`

`rbn_spot_propagation_base` is the first general-purpose view for Workbench,
Tableau, and ad hoc SQL. It starts with `spots_flat`, joins the shared
`activity_5m_buckets` parent, then joins in daily SWPC solar indices and
three-hour K/Kp buckets through relationship-vector fields.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/rbn_spot_propagation_base.sql
```

The view intentionally avoids QRZ profile data. QRZ enrichment is sparse and is
better queried through focused joins from `spots` until QS has broader left join
coverage for optional relationships.

The view exposes `spot_5m_bucket_key`, `activity_5m_id`, `activity_5m_ref`,
`activity_5m_key`, and the normalized bucket metadata from
`activity_5m_buckets`.
Those fields are precomputed by the ingesters so callers can compare RBN spot
activity with submitted contest QSOs without relying on runtime interval
arithmetic.

Useful smoke queries:

```sql
select count(*) as spots
from rbn_spot_propagation_base;
```

```sql
select
  band,
  dx_prefix,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by band, dx_prefix, sfi, kp_index
order by spots desc
limit 50;
```

```sql
select
  activity_5m_key,
  band,
  count(*) as rbn_spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
  and dx_call = 'TI8X'
group by activity_5m_key, band
order by rbn_spots desc
limit 20;
```

## `contest_qso_propagation_base`

`contest_qso_propagation_base` is the matching base view for submitted Cabrillo
contest logs. It starts with `contest_qsos`, joins the `contest_logs` parent
row, and adds daily plus three-hour SWPC propagation context through the
precomputed relationship-vector keys. It also joins the shared
`activity_5m_buckets` parent.

The view exposes `qso_5m_bucket_key`, `activity_5m_id`, `activity_5m_ref`,
`activity_5m_key`, and the normalized bucket metadata from
`activity_5m_buckets` for bucketed comparison against RBN spot activity.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_qso_propagation_base.sql
```

Useful smoke queries:

```sql
select count(*) as qsos
from contest_qso_propagation_base;
```

```sql
select
  station_call,
  station_country,
  claimed_score,
  log_qso_count
from contest_qso_propagation_base
where station_call = 'TI8X'
limit 1;
```

```sql
select
  band,
  worked_continent,
  sfi,
  kp_index,
  count(*) as qsos
from contest_qso_propagation_base
where station_call = 'TI8X'
group by band, worked_continent, sfi, kp_index
order by qsos desc
limit 30;
```

```sql
select
  worked_prefix,
  count(*) as qsos
from contest_qso_propagation_base
where station_call = 'TI8X'
group by worked_prefix
order by qsos desc
limit 25;
```

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

```sql
select
  band,
  dx_prefix,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
  and dx_call = 'TI8X'
group by band, dx_prefix, sfi, kp_index
order by spots desc
limit 50;
```

```sql
select
  spotter_continent,
  dx_continent,
  band,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by spotter_continent, dx_continent, band
order by spots desc
limit 50;
```

## `contest_rbn_activity_5m_base`

`contest_rbn_activity_5m_base` joins submitted Cabrillo QSOs and RBN spots
through the shared `activity_5m_buckets` parent. This avoids a peer-table join
between the two fact tables while keeping the query shape natural for Workbench
and Tableau.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_rbn_activity_5m_base.sql
```

Useful smoke query:

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

Directly joining `contest_qsos` to `spots_flat` on `activity_5m_id` remains a
non-relationship peer-table join and is rejected by QS today. Use
`activity_5m_buckets` or `contest_rbn_activity_5m_base` for the native path.
A future materialized match table can add exact time-delta and frequency-delta
semantics when bucket-level correlation is not enough.
