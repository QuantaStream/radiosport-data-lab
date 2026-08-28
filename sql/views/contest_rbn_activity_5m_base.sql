create or replace view contest_rbn_activity_5m_base as
select
  b.activity_5m_id as activity_5m_id,
  b.activity_5m_key as activity_5m_key,
  b.activity_call as activity_call,
  b.activity_band as activity_band,
  b.activity_mode as activity_mode,
  b.bucket_key as activity_5m_bucket_key,
  b.bucket_start as activity_5m_start,
  q.qso_id as qso_id,
  q.log_id as log_id,
  q.contest_id as contest_id,
  q.qso_at as qso_at,
  q.station_call as station_call,
  q.station_prefix as station_prefix,
  q.station_continent as station_continent,
  q.worked_call as worked_call,
  q.worked_prefix as worked_prefix,
  q.worked_continent as worked_continent,
  q.frequency_khz as qso_frequency_khz,
  q.sent_exchange as sent_exchange,
  q.received_exchange as received_exchange,
  s.spot_id as spot_id,
  s.spotted_at as spotted_at,
  s.spotter_call as spotter_call,
  s.spotter_prefix as spotter_prefix,
  s.spotter_continent as spotter_continent,
  s.dx_call as dx_call,
  s.dx_prefix as dx_prefix,
  s.dx_continent as dx_continent,
  s.frequency_khz as spot_frequency_khz,
  s.signal_db as signal_db,
  s.speed_wpm as speed_wpm,
  d.sfi as sfi,
  d.a_index as a_index,
  d.ap_index as ap_index,
  k.k_index as k_index,
  k.kp_index as kp_index
from activity_5m_buckets as b
inner join contest_qsos as q
  on q.activity_5m_ref = b.activity_5m_id
inner join spots_flat as s
  on s.activity_5m_ref = b.activity_5m_id
inner join swpc_daily_indices as d
  on q.qso_day_key = d.day_key
inner join swpc_k_indices_3h as k
  on q.qso_3h_bucket_key = k.bucket_key;
