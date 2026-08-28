package rbn

import "time"

const (
	DefaultSpotEventType = "rbn_spot"
	FlatSpotEventType    = "rbn_spot_flat"
)

type SpotEvent struct {
	Type string      `json:"type"`
	Data SpotPayload `json:"data"`
}

type SpotPayload struct {
	SpotID           uint64  `json:"spot_id"`
	SpottedAt        string  `json:"spotted_at"`
	SpotDayKey       int     `json:"spot_day_key"`
	Spot3HBucketKey  int     `json:"spot_3h_bucket_key"`
	Spot5MBucketKey  int     `json:"spot_5m_bucket_key"`
	Activity5MID     uint64  `json:"activity_5m_id"`
	Activity5MKey    string  `json:"activity_5m_key"`
	SpotterCall      string  `json:"spotter_call"`
	SpotterPrefix    string  `json:"spotter_prefix"`
	SpotterContinent string  `json:"spotter_continent"`
	DXCall           string  `json:"dx_call"`
	DXPrefix         string  `json:"dx_prefix"`
	DXContinent      string  `json:"dx_continent"`
	FrequencyKHz     float64 `json:"frequency_khz"`
	Band             string  `json:"band"`
	Mode             string  `json:"mode"`
	SignalDB         int     `json:"signal_db"`
	SpeedWPM         int     `json:"speed_wpm"`
	TransmitMode     string  `json:"transmit_mode"`
	Source           string  `json:"source"`
}

func NewSpotEvent(spot Spot) SpotEvent {
	return NewSpotEventWithType(spot, DefaultSpotEventType)
}

func NewSpotEventWithType(spot Spot, eventType string) SpotEvent {
	if eventType == "" {
		eventType = DefaultSpotEventType
	}
	return SpotEvent{
		Type: eventType,
		Data: SpotPayload{
			SpotID:           spot.SpotID,
			SpottedAt:        spot.SpottedAt.UTC().Format(time.RFC3339),
			SpotDayKey:       spot.SpotDayKey,
			Spot3HBucketKey:  spot.Spot3HBucketKey,
			Spot5MBucketKey:  spot.Spot5MBucketKey,
			Activity5MID:     spot.Activity5MID,
			Activity5MKey:    spot.Activity5MKey,
			SpotterCall:      spot.SpotterCall,
			SpotterPrefix:    spot.SpotterPrefix,
			SpotterContinent: spot.SpotterContinent,
			DXCall:           spot.DXCall,
			DXPrefix:         spot.DXPrefix,
			DXContinent:      spot.DXContinent,
			FrequencyKHz:     spot.FrequencyKHz,
			Band:             spot.Band,
			Mode:             spot.Mode,
			SignalDB:         spot.SignalDB,
			SpeedWPM:         spot.SpeedWPM,
			TransmitMode:     spot.TransmitMode,
			Source:           spot.Source,
		},
	}
}
