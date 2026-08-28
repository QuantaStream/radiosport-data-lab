package contestmatch

import "time"

type Event struct {
	Type string  `json:"type"`
	Data Payload `json:"data"`
}

type Payload struct {
	MatchID               uint64  `json:"match_id"`
	ContestID             string  `json:"contest_id"`
	LogID                 string  `json:"log_id"`
	QSOID                 uint64  `json:"qso_id"`
	SpotID                uint64  `json:"spot_id"`
	QSOActivity5MID       uint64  `json:"qso_activity_5m_id"`
	QSOActivity5MKey      string  `json:"qso_activity_5m_key"`
	SpotActivity5MID      uint64  `json:"spot_activity_5m_id"`
	SpotActivity5MKey     string  `json:"spot_activity_5m_key"`
	StationCall           string  `json:"station_call"`
	StationPrefix         string  `json:"station_prefix"`
	StationContinent      string  `json:"station_continent"`
	WorkedCall            string  `json:"worked_call"`
	WorkedPrefix          string  `json:"worked_prefix"`
	WorkedContinent       string  `json:"worked_continent"`
	SpotterCall           string  `json:"spotter_call"`
	SpotterPrefix         string  `json:"spotter_prefix"`
	SpotterContinent      string  `json:"spotter_continent"`
	DXPrefix              string  `json:"dx_prefix"`
	DXContinent           string  `json:"dx_continent"`
	Band                  string  `json:"band"`
	Mode                  string  `json:"mode"`
	QSOAt                 string  `json:"qso_at"`
	SpottedAt             string  `json:"spotted_at"`
	QSOFrequencyKHz       float64 `json:"qso_frequency_khz"`
	SpotFrequencyKHz      float64 `json:"spot_frequency_khz"`
	TimeDeltaSeconds      int     `json:"time_delta_seconds"`
	AbsTimeDeltaSeconds   int     `json:"abs_time_delta_seconds"`
	FrequencyDeltaKHz     float64 `json:"frequency_delta_khz"`
	AbsFrequencyDeltaKHz  float64 `json:"abs_frequency_delta_khz"`
	SignalDB              int     `json:"signal_db"`
	SpeedWPM              int     `json:"speed_wpm"`
	MatchWindowSeconds    int     `json:"match_window_seconds"`
	FrequencyToleranceKHz float64 `json:"frequency_tolerance_khz"`
	SameActivityBucket    int     `json:"same_activity_bucket"`
	MatchKind             string  `json:"match_kind"`
	TimeScore             float64 `json:"time_score"`
	FrequencyScore        float64 `json:"frequency_score"`
	MatchScore            float64 `json:"match_score"`
	MatchRank             int     `json:"match_rank"`
	IsBestMatch           int     `json:"is_best_match"`
	Source                string  `json:"source"`
	LoadedAt              string  `json:"loaded_at"`
}

func NewEvent(match Match) Event {
	return Event{
		Type: EventType,
		Data: Payload{
			MatchID:               match.MatchID,
			ContestID:             match.ContestID,
			LogID:                 match.LogID,
			QSOID:                 match.QSOID,
			SpotID:                match.SpotID,
			QSOActivity5MID:       match.QSOActivity5MID,
			QSOActivity5MKey:      match.QSOActivity5MKey,
			SpotActivity5MID:      match.SpotActivity5MID,
			SpotActivity5MKey:     match.SpotActivity5MKey,
			StationCall:           match.StationCall,
			StationPrefix:         match.StationPrefix,
			StationContinent:      match.StationContinent,
			WorkedCall:            match.WorkedCall,
			WorkedPrefix:          match.WorkedPrefix,
			WorkedContinent:       match.WorkedContinent,
			SpotterCall:           match.SpotterCall,
			SpotterPrefix:         match.SpotterPrefix,
			SpotterContinent:      match.SpotterContinent,
			DXPrefix:              match.DXPrefix,
			DXContinent:           match.DXContinent,
			Band:                  match.Band,
			Mode:                  match.Mode,
			QSOAt:                 formatTime(match.QSOAt),
			SpottedAt:             formatTime(match.SpottedAt),
			QSOFrequencyKHz:       match.QSOFrequencyKHz,
			SpotFrequencyKHz:      match.SpotFrequencyKHz,
			TimeDeltaSeconds:      match.TimeDeltaSeconds,
			AbsTimeDeltaSeconds:   match.AbsTimeDeltaSeconds,
			FrequencyDeltaKHz:     match.FrequencyDeltaKHz,
			AbsFrequencyDeltaKHz:  match.AbsFrequencyDeltaKHz,
			SignalDB:              match.SignalDB,
			SpeedWPM:              match.SpeedWPM,
			MatchWindowSeconds:    match.MatchWindowSeconds,
			FrequencyToleranceKHz: match.FrequencyToleranceKHz,
			SameActivityBucket:    match.SameActivityBucket,
			MatchKind:             match.MatchKind,
			TimeScore:             match.TimeScore,
			FrequencyScore:        match.FrequencyScore,
			MatchScore:            match.MatchScore,
			MatchRank:             match.MatchRank,
			IsBestMatch:           match.IsBestMatch,
			Source:                match.Source,
			LoadedAt:              formatTime(match.LoadedAt),
		},
	}
}

func Events(matches []Match) []interface{} {
	events := make([]interface{}, 0, len(matches))
	for _, match := range matches {
		events = append(events, NewEvent(match))
	}
	return events
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
