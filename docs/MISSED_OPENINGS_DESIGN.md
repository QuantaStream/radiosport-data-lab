# Missed Openings Design

This note captures the first design slice for finding contest openings that were
available but underused.

## Product Question

For a submitted log and a matching RBN spot window:

```text
Where were stations in the target cohort being heard, and did my station miss
that opening?
```

This is different from raw signal reporting. The useful answer needs three
layers:

1. Raw RBN spots, loaded into `spots_flat`.
2. Spotter calibration, loaded into `spotter_profiles`.
3. Station activity summaries, loaded into `station_activity_5m_summaries`.

## First Materialized Shape

`station_activity_5m_summaries` stores one row per:

- DX station callsign
- band
- mode
- five-minute activity bucket
- spotter continent

The builder also writes an `ALL` spotter-continent rollup by default. That keeps
general queries simple while preserving regional reach analysis.

The first summary metrics are:

| Column | Meaning |
| --- | --- |
| `spot_count` | Raw number of RBN spots in the bucket. |
| `distinct_spotters` | Independent receiving stations in the bucket. |
| `distinct_spotter_prefixes` | Rough geographic diversity among spotters. |
| `avg_signal_db` | Average reported signal. |
| `min_signal_db`, `max_signal_db` | Signal bounds. |
| `p50_signal_db`, `p90_signal_db` | Offline signal distribution markers. |
| `reach_score` | Initial composite audibility score. |

The first `reach_score` is intentionally simple:

```text
distinct_spotters * log(1 + distinct_spotter_prefixes) * max(avg_signal_db, 1)
```

This is not the final contest-performance metric. It is a compact way to rank
activity buckets where a station was heard broadly enough to deserve attention.

## Builder

Build summaries from loaded spots:

```bash
go run ./cmd/station-activity-build \
  -target http://127.0.0.1:8088/ingest/json \
  -from 2025-11-29 \
  -to 2025-12-01 \
  -dx-call TI8X \
  -source-table spots_flat
```

Use `-target ""` to write JSONL for inspection instead of posting to the loader.
Omit `-dx-call` when the loaded RBN data contains a peer cohort and you want
activity summaries for every station in that cohort.

## Query Surface

Install the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/station_activity_5m_base.sql
```

Best TI8X audibility buckets:

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

Hour-by-band Tableau source:

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

## Cohort Path

The next real missed-opening table should compare a station with a peer set.
For each band/mode/five-minute bucket, materialize:

- target station activity
- peer cohort activity
- spotter-continent reach
- spotter-calibrated or normalized signal
- whether the target was absent, weak, normal, or strong

That table can be built offline from `station_activity_5m_summaries` plus a
future `contest_station_cohorts` table. It avoids runtime interval joins and
keeps Tableau focused on analysis rather than query synthesis.

## Product Improvement Opportunities

- The loader should eventually support derived-table jobs that read from QS and
  post back into QS as a first-class pipeline pattern.
- Cohort definitions need to be explicit: entry category, geography, power, and
  station class all matter.
- Spotter profiles should evolve toward band/mode-specific calibration before
  any weighted SNR score becomes a product recommendation.
- Missed-opening views should expose precomputed date parts so Tableau does not
  need generated function-heavy SQL for common worksheets.
