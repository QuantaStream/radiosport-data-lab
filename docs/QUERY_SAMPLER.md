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
select count(*) as spotter_profile_count from spotter_profiles;
select count(*) as station_activity_count from station_activity_5m_summaries;
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

Calibrated signal by band:

```sql
select
  station_call,
  band,
  avg(raw_signal_db) as avg_snr,
  avg(normalized_signal_db) as avg_normalized_snr,
  sum(calibrated_reach_numerator) / sum(calibrated_reach_weight) as calibrated_reach_snr,
  count(*) as qsos
from contest_competitiveness_signal_base
group by station_call, band
order by station_call, band;
```

Logged QSO volume by station, band, and hour:

```sql
select
  station_call,
  band,
  qso_hour,
  count(*) as logged_qsos
from contest_competitiveness_qso_base
group by station_call, band, qso_hour
order by station_call, band, qso_hour;
```

Calibrated signal by receiving continent:

```sql
select
  station_call,
  band,
  receiving_continent,
  avg(raw_signal_db) as avg_snr,
  sum(calibrated_reach_numerator) / sum(calibrated_reach_weight) as calibrated_reach_snr,
  count(*) as matched_qsos
from contest_competitiveness_signal_base
group by station_call, band, receiving_continent
order by station_call, band, matched_qsos desc;
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

## Spotter Profiles

Top spotters in the current calibration window:

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
  distinct_dx_calls,
  avg_signal_db,
  p50_signal_db,
  p90_signal_db,
  spotter_weight,
  profile_quality
from spotter_profile_base
order by total_spots desc
limit 25;
```

Spotter map starting point:

```sql
select
  spotter_call,
  country_name,
  latitude,
  longitude,
  total_spots,
  avg_signal_db,
  spotter_weight
from spotter_profile_base
where geo_confidence = 'COUNTRY_CENTROID'
order by total_spots desc
limit 100;
```

Spotter calibration shape by continent:

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

These rows are calibration metadata. The first weighting policy is deliberately
simple: high-volume spotters get `spotter_weight = 1 / sqrt(total_spots)`, while
`normalization_offset_db` starts as the spotter's p50 reported signal.

## Station Activity

Best five-minute activity buckets for one station:

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

Hour-by-band activity shape for Tableau:

```sql
select
  bucket_hour,
  band,
  spotter_continent,
  sum(spot_count) as spots,
  avg(avg_signal_db) as avg_signal_db,
  avg(reach_score) as avg_reach_score
from station_activity_5m_base
where dx_call = 'TI8X'
group by bucket_hour, band, spotter_continent
order by bucket_hour, band, spots desc
limit 200;
```

These rows are station-time summaries, not raw spot rows. They are meant to make
missed-opening and cohort-comparison queries practical without asking Tableau to
generate interval joins or large high-cardinality groupings.

## Tableau-Friendly Starts

Use these views as first Tableau logical tables:

| View | Good First Worksheet |
| --- | --- |
| `contest_best_spot_match_base` | `qso_hour` by `band`, colored by `avg(signal_db)` |
| `contest_spot_match_base` | spotter quality and match-rank inspection |
| `contest_competitiveness_qso_base` | logged QSO volume by `qso_hour`, `band`, and `station_call` |
| `contest_competitiveness_signal_base` | calibrated signal reach by station, band, hour, and receiving continent |
| `contest_qso_propagation_base` | QSO volume by band, worked continent, SFI, Kp |
| `rbn_spot_propagation_base` | RBN spot volume by band, prefix, SFI, Kp |
| `spotter_profile_base` | spotter volume, baseline signal, and calibration quality |
| `station_activity_5m_base` | station audibility by five-minute bucket, band, and receiving continent |

Calculated fields that are already exposed as columns in the views should be
used directly in Tableau when possible. For example, prefer `qso_hour` over a
generated Tableau `DATEPART` expression during early compatibility testing.
