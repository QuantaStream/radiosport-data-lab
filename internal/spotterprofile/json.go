package spotterprofile

import "time"

type NodeEvent struct {
	Type string      `json:"type"`
	Data NodePayload `json:"data"`
}

type NodePayload struct {
	SpotterCall        string  `json:"spotter_call"`
	SpotterPrefix      string  `json:"spotter_prefix"`
	SpotterContinent   string  `json:"spotter_continent"`
	Grid               string  `json:"grid"`
	DXCCID             int     `json:"dxcc_id"`
	CountryName        string  `json:"country_name"`
	CQZone             int     `json:"cq_zone"`
	ITUZone            int     `json:"itu_zone"`
	Latitude           float64 `json:"latitude"`
	Longitude          float64 `json:"longitude"`
	GeoSource          string  `json:"geo_source"`
	GeoConfidence      string  `json:"geo_confidence"`
	Bands              string  `json:"bands"`
	SkimmerSoftware    string  `json:"skimmer_software"`
	AggregatorSoftware string  `json:"aggregator_software"`
	FirstSeenAt        string  `json:"first_seen_at"`
	LastSeenAt         string  `json:"last_seen_at"`
	Source             string  `json:"source"`
	LoadedAt           string  `json:"loaded_at"`
}

type ProfileEvent struct {
	Type string         `json:"type"`
	Data ProfilePayload `json:"data"`
}

type SnapshotEvent struct {
	Type string         `json:"type"`
	Data ProfilePayload `json:"data"`
}

type ProfilePayload struct {
	ProfileID             string  `json:"profile_id"`
	SpotterCall           string  `json:"spotter_call"`
	SpotterPrefix         string  `json:"spotter_prefix"`
	SpotterContinent      string  `json:"spotter_continent"`
	CountryName           string  `json:"country_name"`
	CQZone                int     `json:"cq_zone"`
	ITUZone               int     `json:"itu_zone"`
	Latitude              float64 `json:"latitude"`
	Longitude             float64 `json:"longitude"`
	GeoSource             string  `json:"geo_source"`
	GeoConfidence         string  `json:"geo_confidence"`
	ProfileKind           string  `json:"profile_kind"`
	WindowStart           string  `json:"window_start"`
	WindowEnd             string  `json:"window_end"`
	Band                  string  `json:"band"`
	Mode                  string  `json:"mode"`
	TotalSpots            int     `json:"total_spots"`
	ActiveDays            int     `json:"active_days"`
	ActiveHours           int     `json:"active_hours"`
	DistinctDXCalls       int     `json:"distinct_dx_calls"`
	DistinctDXPrefixes    int     `json:"distinct_dx_prefixes"`
	AvgSignalDB           float64 `json:"avg_signal_db"`
	MinSignalDB           int     `json:"min_signal_db"`
	MaxSignalDB           int     `json:"max_signal_db"`
	P50SignalDB           float64 `json:"p50_signal_db"`
	P90SignalDB           float64 `json:"p90_signal_db"`
	VolumeWeight          float64 `json:"volume_weight"`
	SpotterWeight         float64 `json:"spotter_weight"`
	NormalizationOffsetDB float64 `json:"normalization_offset_db"`
	ProfileQuality        string  `json:"profile_quality"`
	SourceTable           string  `json:"source_table"`
	ComputedAt            string  `json:"computed_at"`
	ProfileComputedAt     string  `json:"profile_computed_at"`
}

func NewNodeEvent(summary Summary) NodeEvent {
	return NodeEvent{
		Type: NodeEventType,
		Data: NodePayload{
			SpotterCall:        summary.SpotterCall,
			SpotterPrefix:      summary.SpotterPrefix,
			SpotterContinent:   summary.SpotterContinent,
			Grid:               "",
			DXCCID:             0,
			CountryName:        summary.CountryName,
			CQZone:             summary.CQZone,
			ITUZone:            summary.ITUZone,
			Latitude:           summary.Latitude,
			Longitude:          summary.Longitude,
			GeoSource:          summary.GeoSource,
			GeoConfidence:      summary.GeoConfidence,
			Bands:              "UNKNOWN",
			SkimmerSoftware:    "UNKNOWN",
			AggregatorSoftware: "UNKNOWN",
			FirstSeenAt:        formatTime(summary.WindowStart),
			LastSeenAt:         formatTime(summary.WindowEnd),
			Source:             "spots_flat_profile_build",
			LoadedAt:           formatTime(summary.ComputedAt),
		},
	}
}

func NewProfileEvent(summary Summary) ProfileEvent {
	return ProfileEvent{
		Type: ProfileEventType,
		Data: profilePayload(summary),
	}
}

func NewSnapshotEvent(summary Summary) SnapshotEvent {
	return SnapshotEvent{
		Type: SnapshotEventType,
		Data: profilePayload(summary),
	}
}

func profilePayload(summary Summary) ProfilePayload {
	return ProfilePayload{
		ProfileID:             summary.ProfileID,
		SpotterCall:           summary.SpotterCall,
		SpotterPrefix:         summary.SpotterPrefix,
		SpotterContinent:      summary.SpotterContinent,
		CountryName:           summary.CountryName,
		CQZone:                summary.CQZone,
		ITUZone:               summary.ITUZone,
		Latitude:              summary.Latitude,
		Longitude:             summary.Longitude,
		GeoSource:             summary.GeoSource,
		GeoConfidence:         summary.GeoConfidence,
		ProfileKind:           summary.ProfileKind,
		WindowStart:           formatTime(summary.WindowStart),
		WindowEnd:             formatTime(summary.WindowEnd),
		Band:                  summary.Band,
		Mode:                  summary.Mode,
		TotalSpots:            summary.TotalSpots,
		ActiveDays:            summary.ActiveDays,
		ActiveHours:           summary.ActiveHours,
		DistinctDXCalls:       summary.DistinctDXCalls,
		DistinctDXPrefixes:    summary.DistinctDXPrefixes,
		AvgSignalDB:           summary.AvgSignalDB,
		MinSignalDB:           summary.MinSignalDB,
		MaxSignalDB:           summary.MaxSignalDB,
		P50SignalDB:           summary.P50SignalDB,
		P90SignalDB:           summary.P90SignalDB,
		VolumeWeight:          summary.SpotterWeight,
		SpotterWeight:         summary.SpotterWeight,
		NormalizationOffsetDB: summary.NormalizationOffsetDB,
		ProfileQuality:        summary.ProfileQuality,
		SourceTable:           summary.SourceTable,
		ComputedAt:            formatTime(summary.ComputedAt),
		ProfileComputedAt:     formatTime(summary.ComputedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
