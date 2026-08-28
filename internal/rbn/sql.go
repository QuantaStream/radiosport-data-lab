package rbn

import "time"

const SpotInsertSQL = `insert into spots (
  spot_id,
  spotted_at,
  spot_day_key,
  spot_3h_bucket_key,
  spotter_call,
  spotter_prefix,
  spotter_continent,
  dx_call,
  dx_prefix,
  dx_continent,
  frequency_khz,
  band,
  mode,
  signal_db,
  speed_wpm,
  transmit_mode,
  source,
  spot_5m_bucket_key,
  activity_5m_id,
  activity_5m_key
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func SpotSQLArgs(spot Spot) []interface{} {
	return []interface{}{
		spot.SpotID,
		formatSQLTime(spot.SpottedAt),
		spot.SpotDayKey,
		spot.Spot3HBucketKey,
		spot.SpotterCall,
		spot.SpotterPrefix,
		spot.SpotterContinent,
		spot.DXCall,
		spot.DXPrefix,
		spot.DXContinent,
		spot.FrequencyKHz,
		spot.Band,
		spot.Mode,
		spot.SignalDB,
		spot.SpeedWPM,
		spot.TransmitMode,
		spot.Source,
		spot.Spot5MBucketKey,
		spot.Activity5MID,
		spot.Activity5MKey,
	}
}

func formatSQLTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}
