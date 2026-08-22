package rbn

import "time"

const SpotInsertSQL = `insert into spots (
  spot_id,
  spotted_at,
  spotter_call,
  spotter_prefix,
  spotter_continent,
  dx_call,
  dx_prefix,
  dx_continent,
  frequency_hz,
  band,
  mode,
  signal_db,
  speed_wpm,
  transmit_mode,
  source
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func SpotSQLArgs(spot Spot) []interface{} {
	return []interface{}{
		spot.SpotID,
		formatSQLTime(spot.SpottedAt),
		spot.SpotterCall,
		spot.SpotterPrefix,
		spot.SpotterContinent,
		spot.DXCall,
		spot.DXPrefix,
		spot.DXContinent,
		spot.FrequencyHz,
		spot.Band,
		spot.Mode,
		spot.SignalDB,
		spot.SpeedWPM,
		spot.TransmitMode,
		spot.Source,
	}
}

func formatSQLTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}
