select
  station_call,
  category_operator,
  category_assisted,
  category_band,
  category_power,
  category_mode,
  category_transmitter,
  claimed_score,
  log_qso_count,
  count(*) as loaded_qsos
from contest_competitiveness_qso_base
where category_mode = 'SSB'
group by
  station_call,
  category_operator,
  category_assisted,
  category_band,
  category_power,
  category_mode,
  category_transmitter,
  claimed_score,
  log_qso_count
order by claimed_score desc, station_call;

select
  station_call,
  qso_hour,
  band,
  count(*) as qsos
from contest_competitiveness_qso_base
where category_mode = 'SSB'
group by station_call, qso_hour, band
order by station_call, qso_hour, qsos desc;

select
  station_call,
  qso_hour,
  band,
  worked_continent,
  count(*) as qsos
from contest_competitiveness_qso_base
where category_mode = 'SSB'
  and band in ('10m', '15m')
group by station_call, qso_hour, band, worked_continent
order by qso_hour, band, qsos desc;

select
  station_call,
  band,
  received_exchange as cq_zone,
  count(*) as qsos
from contest_competitiveness_qso_base
where category_mode = 'SSB'
  and received_exchange in
    ('17', '18', '20', '21', '22', '23', '28', '29',
     '34', '35', '36', '37', '39', '40')
group by station_call, band, received_exchange
order by station_call, band, received_exchange;

select
  station_call,
  qso_day,
  qso_hour,
  sfi,
  k_index,
  kp_index,
  count(*) as qsos
from contest_competitiveness_qso_base
where category_mode = 'SSB'
group by station_call, qso_day, qso_hour, sfi, k_index, kp_index
order by station_call, qso_day, qso_hour;

select
  dx_call,
  band,
  spotter_continent,
  count(*) as spots,
  avg(signal_db) as avg_signal_db
from rbn_spot_propagation_base
where spot_day_key in (20231022, 20231104, 20231105)
group by dx_call, band, spotter_continent
order by dx_call, spots desc, band;

select
  dx_call,
  band,
  spotter_continent,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spot_day_key in (20231022, 20231104, 20231105)
group by dx_call, band, spotter_continent, sfi, kp_index
order by dx_call, spots desc, band;

select
  dx_call,
  bucket_start,
  bucket_day,
  bucket_hour,
  band,
  spotter_continent,
  spot_count,
  distinct_spotters,
  avg_signal_db,
  reach_score
from station_activity_5m_base
order by reach_score desc
limit 100;
