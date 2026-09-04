-- CQ WW CW 2025 SOAB HP battle: reproducible QuantaStream queries.
-- Load output/battle-timeline.jsonl into cqww_battle_buckets first.

-- Four-way checkpoint table. Run once for each documented timestamp.
select
  bucket_end,
  station_call,
  cumulative_counted_qsos,
  cumulative_points,
  cumulative_countries,
  cumulative_zones,
  cumulative_multipliers,
  cumulative_score,
  score_behind_leader
from cqww_battle_buckets
where bucket_end = todate('2025-11-29 12:00:00')
order by cumulative_score desc;

select
  bucket_end,
  station_call,
  cumulative_counted_qsos,
  cumulative_points,
  cumulative_multipliers,
  cumulative_score,
  score_behind_leader
from cqww_battle_buckets
where bucket_end = todate('2025-11-30 00:00:00')
order by cumulative_score desc;

select
  bucket_end,
  station_call,
  cumulative_counted_qsos,
  cumulative_points,
  cumulative_multipliers,
  cumulative_score,
  score_behind_leader
from cqww_battle_buckets
where bucket_end = todate('2025-11-30 12:00:00')
order by cumulative_score desc;

select
  bucket_end,
  station_call,
  cumulative_counted_qsos,
  cumulative_points,
  cumulative_countries,
  cumulative_zones,
  cumulative_multipliers,
  cumulative_score,
  score_behind_leader
from cqww_battle_buckets
where bucket_end = todate('2025-12-01 00:00:00')
order by cumulative_score desc;

-- Total unique-QSO production by band. Maritime-mobile zone-only contacts are
-- included in bucket QSO counts on their logged bands.
select
  station_call,
  sum(bucket_10m) as qsos_10m,
  sum(bucket_15m) as qsos_15m,
  sum(bucket_20m) as qsos_20m,
  sum(bucket_40m) as qsos_40m,
  sum(bucket_80m) as qsos_80m,
  sum(bucket_160m) as qsos_160m
from cqww_battle_buckets
group by station_call
order by station_call;

-- The reconstructed race at five-minute resolution.
select
  bucket_end,
  station_call,
  leader_call,
  cumulative_score,
  score_behind_leader,
  leader_margin,
  bucket_counted_qsos,
  bucket_points,
  bucket_countries,
  bucket_zones
from cqww_battle_buckets
order by bucket_end, cumulative_score desc;
