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
  station_latitude,
  station_longitude,
  station_geo_confidence,
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
`activity_5m_buckets` or `contest_rbn_activity_5m_base` for the native bucket
path.

## `contest_spot_match_base`

`contest_spot_match_base` is the exact match view over materialized
`contest_spot_matches` rows. It is the richer analysis surface when five-minute
bucket correlation is not enough because it includes time delta, frequency
delta, score/rank, signal, spotter, and SWPC context.

The match views also expose Tableau-friendly UTC date parts:
`qso_year`, `qso_month`, `qso_day`, `qso_day_of_week`, `qso_hour`,
`spotted_year`, `spotted_month`, `spotted_day`, `spotted_day_of_week`, and
`spotted_hour`. These keep common worksheets from needing generated date-part
SQL for simple hour, day, and month heatmaps.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_spot_match_base.sql
```

Create or refresh the best-match convenience view after the base view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_best_spot_match_base.sql
```

Useful smoke query:

```sql
select
  band,
  match_kind,
  count(*) as matches
from contest_spot_match_base
where station_call = 'TI8X'
group by band, match_kind
order by matches desc
limit 20;
```

Closest spotter examples:

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
  time_score,
  frequency_score,
  spotter_call,
  signal_db
from contest_spot_match_base
where station_call = 'TI8X'
  and abs_time_delta_seconds <= 60
order by match_score desc
limit 30;
```

Best match per QSO:

```sql
select
  qso_id,
  qso_at,
  qso_hour,
  band,
  spotter_call,
  spotted_at,
  time_delta_seconds,
  abs_frequency_delta_khz,
  match_score
from contest_best_spot_match_base
where station_call = 'TI8X'
order by qso_at
limit 30;
```

## `spotter_profile_base`

`spotter_profile_base` is the current calibration surface for RBN receiving
stations. The first implementation is computed from `spots_flat` and stores one
current profile row per `spotter_call`, plus immutable rows in
`spotter_profile_snapshots`.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/spotter_profile_base.sql
```

Build or refresh profiles from the currently loaded spot window:

```bash
go run ./cmd/spotter-profile-build \
  -target http://127.0.0.1:8088/ingest/json \
  -from 2025-11-29 \
  -to 2025-12-01 \
  -profile-kind contest \
  -source-table spots_flat \
  -cty-dat data/cty/cty.dat
```

Useful smoke queries:

```sql
select
  spotter_call,
  spotter_continent,
  country_name,
  latitude,
  longitude,
  geo_confidence,
  total_spots,
  active_hours,
  avg_signal_db,
  p50_signal_db,
  p90_signal_db,
  spotter_weight,
  profile_quality
from spotter_profile_base
order by total_spots desc
limit 25;
```

```sql
select
  spotter_call,
  country_name,
  latitude,
  longitude,
  geo_source,
  geo_confidence,
  total_spots
from spotter_profile_base
where geo_confidence = 'COUNTRY_CENTROID'
order by total_spots desc
limit 25;
```

```sql
select
  spotter_continent,
  profile_quality,
  count(*) as spotters,
  sum(total_spots) as spots,
  avg(avg_signal_db) as avg_spotter_baseline
from spotter_profile_base
group by spotter_continent, profile_quality
order by spots desc;
```

Use `spotter_weight` as the denominator weight for early weighted-SNR
experiments. Runtime joins from match views to `spotter_profiles` are not the
preferred QS path yet; materialize weighted match rows or add profile references
to future match rows when the metric settles.

## `station_activity_5m_base`

`station_activity_5m_base` is the first missed-openings foundation view. It is
materialized from `spots_flat` by `cmd/station-activity-build` and stores one
row per DX station, band, mode, five-minute bucket, and spotter continent. The
builder also emits an `ALL` continent rollup for each bucket by default.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/station_activity_5m_base.sql
```

Build station activity from the currently loaded spot window:

```bash
go run ./cmd/station-activity-build \
  -target http://127.0.0.1:8088/ingest/json \
  -from 2025-11-29 \
  -to 2025-12-01 \
  -dx-call TI8X \
  -source-table spots_flat
```

Useful smoke queries:

```sql
select
  dx_call,
  band,
  bucket_hour,
  spotter_continent,
  spot_count,
  distinct_spotters,
  avg_signal_db,
  reach_score
from station_activity_5m_base
where dx_call = 'TI8X'
order by reach_score desc
limit 25;
```

```sql
select
  bucket_hour,
  band,
  spotter_continent,
  sum(spot_count) as spots,
  sum(distinct_spotters) as spotter_observations,
  avg(avg_signal_db) as avg_signal_db,
  avg(reach_score) as avg_reach_score
from station_activity_5m_base
where dx_call = 'TI8X'
group by bucket_hour, band, spotter_continent
order by bucket_hour, band, spots desc
limit 200;
```

The next missed-openings step is a peer/cohort comparison table that can mark
activity buckets where similar stations were broadly heard but the target
station was absent or weak.
