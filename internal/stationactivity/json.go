package stationactivity

import "time"

type Event struct {
	Type string  `json:"type"`
	Data Payload `json:"data"`
}

type Payload struct {
	SummaryID               uint64  `json:"summary_id"`
	SummaryKey              string  `json:"summary_key"`
	Activity5MID            uint64  `json:"activity_5m_id"`
	Activity5MKey           string  `json:"activity_5m_key"`
	BucketKey               int     `json:"bucket_key"`
	BucketStart             string  `json:"bucket_start"`
	DXCall                  string  `json:"dx_call"`
	DXPrefix                string  `json:"dx_prefix"`
	DXContinent             string  `json:"dx_continent"`
	Band                    string  `json:"band"`
	Mode                    string  `json:"mode"`
	SpotterContinent        string  `json:"spotter_continent"`
	SpotCount               int     `json:"spot_count"`
	DistinctSpotters        int     `json:"distinct_spotters"`
	DistinctSpotterPrefixes int     `json:"distinct_spotter_prefixes"`
	AvgSignalDB             float64 `json:"avg_signal_db"`
	MinSignalDB             int     `json:"min_signal_db"`
	MaxSignalDB             int     `json:"max_signal_db"`
	P50SignalDB             float64 `json:"p50_signal_db"`
	P90SignalDB             float64 `json:"p90_signal_db"`
	ReachScore              float64 `json:"reach_score"`
	SourceTable             string  `json:"source_table"`
	ComputedAt              string  `json:"computed_at"`
}

func NewEvent(summary Summary) Event {
	return Event{
		Type: EventType,
		Data: Payload{
			SummaryID:               summary.SummaryID,
			SummaryKey:              summary.SummaryKey,
			Activity5MID:            summary.Activity5MID,
			Activity5MKey:           summary.Activity5MKey,
			BucketKey:               summary.BucketKey,
			BucketStart:             formatTime(summary.BucketStart),
			DXCall:                  summary.DXCall,
			DXPrefix:                summary.DXPrefix,
			DXContinent:             summary.DXContinent,
			Band:                    summary.Band,
			Mode:                    summary.Mode,
			SpotterContinent:        summary.SpotterContinent,
			SpotCount:               summary.SpotCount,
			DistinctSpotters:        summary.DistinctSpotters,
			DistinctSpotterPrefixes: summary.DistinctSpotterPrefixes,
			AvgSignalDB:             summary.AvgSignalDB,
			MinSignalDB:             summary.MinSignalDB,
			MaxSignalDB:             summary.MaxSignalDB,
			P50SignalDB:             summary.P50SignalDB,
			P90SignalDB:             summary.P90SignalDB,
			ReachScore:              summary.ReachScore,
			SourceTable:             summary.SourceTable,
			ComputedAt:              formatTime(summary.ComputedAt),
		},
	}
}

func Events(summaries []Summary) []interface{} {
	events := make([]interface{}, 0, len(summaries))
	for _, summary := range summaries {
		events = append(events, NewEvent(summary))
	}
	return events
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
