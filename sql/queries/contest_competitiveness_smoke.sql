select
  station_call,
  band,
  avg(raw_signal_db) as avg_raw_snr,
  avg(normalized_signal_db) as avg_normalized_snr,
  sum(calibrated_reach_numerator) / sum(calibrated_reach_weight) as calibrated_reach_snr,
  count(*) as matched_qsos
from contest_competitiveness_signal_base
group by station_call, band
order by station_call, band;

select
  station_call,
  band,
  qso_hour,
  count(*) as logged_qsos
from contest_competitiveness_qso_base
group by station_call, band, qso_hour
order by station_call, band, qso_hour
limit 200;

select
  station_call,
  band,
  qso_hour,
  receiving_continent,
  avg(raw_signal_db) as avg_raw_snr,
  avg(normalized_signal_db) as avg_normalized_snr,
  sum(calibrated_reach_numerator) / sum(calibrated_reach_weight) as calibrated_reach_snr,
  count(*) as matched_qsos
from contest_competitiveness_signal_base
group by station_call, band, qso_hour, receiving_continent
order by station_call, band, qso_hour, matched_qsos desc
limit 200;
