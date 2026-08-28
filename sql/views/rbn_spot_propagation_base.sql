create or replace view rbn_spot_propagation_base as
select
  s.spot_id as spot_id,
  s.spotted_at as spotted_at,
  s.spot_day_key as spot_day_key,
  s.spot_3h_bucket_key as spot_3h_bucket_key,
  s.spot_5m_bucket_key as spot_5m_bucket_key,
  s.activity_5m_id as activity_5m_id,
  s.activity_5m_key as activity_5m_key,
  s.spot_day_ref as spot_day_ref,
  s.spot_3h_bucket_ref as spot_3h_bucket_ref,
  s.spotter_call as spotter_call,
  s.spotter_prefix as spotter_prefix,
  s.spotter_continent as spotter_continent,
  s.dx_call as dx_call,
  s.dx_prefix as dx_prefix,
  s.dx_continent as dx_continent,
  s.frequency_khz as frequency_khz,
  s.band as band,
  s.mode as mode,
  s.signal_db as signal_db,
  s.speed_wpm as speed_wpm,
  s.transmit_mode as transmit_mode,
  s.source as spot_source,
  d.observed_date as swpc_observed_date,
  d.a_index as a_index,
  d.ap_index as ap_index,
  d.sfi as sfi,
  d.sunspot_number as sunspot_number,
  d.source as swpc_daily_source,
  k.bucket_start as swpc_bucket_start,
  k.k_index as k_index,
  k.kp_index as kp_index,
  k.source as swpc_k_source
from spots_flat as s
inner join swpc_daily_indices as d
  on s.spot_day_ref = d.day_key
inner join swpc_k_indices_3h as k
  on s.spot_3h_bucket_ref = k.bucket_key;
