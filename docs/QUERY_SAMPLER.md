# Query Sampler

These are copy/paste SQL examples for Workbench, the MySQL CLI, and Tableau
custom SQL experiments against the RadioSport Data Lab.

Connect locally:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta
```

## Inventory

```sql
show tables;
```

```sql
select count(*) as spots_flat_count from spots_flat;
select count(*) as qso_count from contest_qsos;
select count(*) as match_count from contest_spot_matches;
select count(*) as swpc_daily_count from swpc_daily_indices;
select count(*) as swpc_k_count from swpc_k_indices_3h;
```

```sql
select
  min(spotted_at) as first_spot,
  max(spotted_at) as last_spot,
  count(*) as spots
from spots_flat;
```

## RBN Spot Exploration

Top bands and modes:

```sql
select band, mode, count(*) as spots
from spots_flat
group by band, mode
order by spots desc
limit 20;
```

Top DXCC prefixes:

```sql
select dx_prefix, count(*) as spots
from spots_flat
group by dx_prefix
order by spots desc
limit 25;
```

Spotter-to-DX continent matrix:

```sql
select spotter_continent, dx_continent, count(*) as spots
from spots_flat
group by spotter_continent, dx_continent
order by spots desc;
```

Signal by prefix on 20m:

```sql
select dx_prefix, avg(signal_db) as avg_signal, count(*) as spots
from spots_flat
where band = '20m'
group by dx_prefix
order by spots desc
limit 20;
```

Callsign prefix filter:

```sql
select dx_call, spotted_at, frequency_khz, band, mode, signal_db, spotter_call
from spots_flat
where dx_call like 'N7%'
order by spotted_at desc
limit 50;
```

Long portable callsign filter:

```sql
select dx_call, spotted_at, frequency_khz, band, mode, signal_db, spotter_call
from spots_flat
where dx_call like 'OE/DL7UZO%'
order by spotted_at desc
limit 20;
```

`StringLexBSI` is strongest for equality, prefix filtering, selective
projection, and long-string residual validation. Full high-cardinality grouped
aggregates over callsigns are useful, but they are a product-tuning area:

```sql
select dx_call, count(*) as spots
from spots_flat
group by dx_call
order by spots desc
limit 20;
```

Use categorical fields when exploring fast Top-N behavior:

```sql
select topn(dx_prefix, 25) from spots_flat;
```

```sql
select dx_prefix, count(*) as spots
from spots_flat
group by dx_prefix
order by spots desc
limit 25;
```

## Propagation Views

RBN spots with SWPC context:

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

Focused TI8X spot activity by propagation context:

```sql
select
  band,
  dx_continent,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
  and dx_call = 'TI8X'
group by band, dx_continent, sfi, kp_index
order by spots desc
limit 50;
```

Submitted QSOs with SWPC context:

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

## QSO-To-Spot Matching

Exact matches by band:

```sql
select band, match_kind, count(*) as matches
from contest_spot_match_base
where station_call = 'TI8X'
group by band, match_kind
order by matches desc
limit 20;
```

Best RBN match per QSO by band:

```sql
select band, avg(signal_db) as avg_snr, count(*) as qsos
from contest_best_spot_match_base
where station_call = 'TI8X'
group by band
order by qsos desc;
```

Best matches by spotter:

```sql
select spotter_call, avg(signal_db) as avg_snr, count(*) as qsos
from contest_best_spot_match_base
where station_call = 'TI8X'
group by spotter_call
order by qsos desc
limit 25;
```

Band and receiving-continent signal shape:

```sql
select band, spotter_continent, avg(signal_db) as avg_snr, count(*) as qsos
from contest_best_spot_match_base
where station_call = 'TI8X'
group by band, spotter_continent
order by band, qsos desc;
```

Band and worked-continent signal shape:

```sql
select band, worked_continent, avg(signal_db) as avg_snr, count(*) as qsos
from contest_best_spot_match_base
where station_call = 'TI8X'
group by band, worked_continent
order by band, qsos desc;
```

Hour-by-band-by-continent heatmap source:

```sql
select
  qso_hour,
  band,
  spotter_continent,
  avg(signal_db) as avg_snr,
  count(*) as qsos
from contest_best_spot_match_base
where station_call = 'TI8X'
group by qso_hour, band, spotter_continent
order by qso_hour, band, qsos desc
limit 1000;
```

Closest high-scoring spot examples:

```sql
select
  qso_at,
  worked_call,
  band,
  qso_frequency_khz,
  spotted_at,
  spotter_call,
  spot_frequency_khz,
  signal_db,
  abs_time_delta_seconds,
  abs_frequency_delta_khz,
  match_score
from contest_best_spot_match_base
where station_call = 'TI8X'
order by match_score desc
limit 30;
```

## Activity Buckets

Bucket density from raw RBN spots:

```sql
select activity_5m_key, band, count(*) as rbn_spots
from spots_flat
group by activity_5m_key, band
order by rbn_spots desc
limit 20;
```

Native relationship-vector bucket match:

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

Use `contest_spot_match_base` for exact materialized time/frequency matches.
Use `contest_rbn_activity_5m_base` for broader bucket-level activity analysis.

## Tableau-Friendly Starts

Use these views as first Tableau logical tables:

| View | Good First Worksheet |
| --- | --- |
| `contest_best_spot_match_base` | `qso_hour` by `band`, colored by `avg(signal_db)` |
| `contest_spot_match_base` | spotter quality and match-rank inspection |
| `contest_qso_propagation_base` | QSO volume by band, worked continent, SFI, Kp |
| `rbn_spot_propagation_base` | RBN spot volume by band, prefix, SFI, Kp |

Calculated fields that are already exposed as columns in the views should be
used directly in Tableau when possible. For example, prefer `qso_hour` over a
generated Tableau `DATEPART` expression during early compatibility testing.

