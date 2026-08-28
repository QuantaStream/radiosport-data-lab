create or replace view contest_qso_propagation_base as
select
  q.qso_id as qso_id,
  q.log_id as log_id,
  q.contest_id as contest_id,
  q.qso_at as qso_at,
  q.qso_day_key as qso_day_key,
  q.qso_3h_bucket_key as qso_3h_bucket_key,
  q.station_call as station_call,
  q.station_prefix as station_prefix,
  q.station_continent as station_continent,
  l.station_country as station_country,
  l.cq_zone as station_cq_zone,
  l.itu_zone as station_itu_zone,
  l.category_operator as category_operator,
  l.category_assisted as category_assisted,
  l.category_band as category_band,
  l.category_power as category_power,
  l.category_mode as category_mode,
  l.category_transmitter as category_transmitter,
  l.claimed_score as claimed_score,
  l.qso_count as log_qso_count,
  l.scope_region as scope_region,
  q.worked_call as worked_call,
  q.worked_prefix as worked_prefix,
  q.worked_continent as worked_continent,
  q.frequency_khz as frequency_khz,
  q.band as band,
  q.mode as mode,
  q.sent_exchange as sent_exchange,
  q.received_exchange as received_exchange,
  q.source_file as qso_source_file,
  l.source_file as log_source_file,
  l.loaded_at as log_loaded_at,
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
from contest_qsos as q
inner join contest_logs as l
  on q.log_id = l.log_id
inner join swpc_daily_indices as d
  on q.qso_day_key = d.day_key
inner join swpc_k_indices_3h as k
  on q.qso_3h_bucket_key = k.bucket_key;
