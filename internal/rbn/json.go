package rbn

import "time"

type SpotEvent struct {
	Type string      `json:"type"`
	Data SpotPayload `json:"data"`
}

type SpotPayload struct {
	SpotID           uint64 `json:"spot_id"`
	SpottedAt        string `json:"spotted_at"`
	SpotterCall      string `json:"spotter_call"`
	SpotterPrefix    string `json:"spotter_prefix"`
	SpotterContinent string `json:"spotter_continent"`
	DXCall           string `json:"dx_call"`
	DXPrefix         string `json:"dx_prefix"`
	DXContinent      string `json:"dx_continent"`
	FrequencyHz      int64  `json:"frequency_hz"`
	Band             string `json:"band"`
	Mode             string `json:"mode"`
	SignalDB         int    `json:"signal_db"`
	SpeedWPM         int    `json:"speed_wpm"`
	TransmitMode     string `json:"transmit_mode"`
	Source           string `json:"source"`
}

func NewSpotEvent(spot Spot) SpotEvent {
	return SpotEvent{
		Type: "rbn_spot",
		Data: SpotPayload{
			SpotID:           spot.SpotID,
			SpottedAt:        spot.SpottedAt.UTC().Format(time.RFC3339),
			SpotterCall:      spot.SpotterCall,
			SpotterPrefix:    spot.SpotterPrefix,
			SpotterContinent: spot.SpotterContinent,
			DXCall:           spot.DXCall,
			DXPrefix:         spot.DXPrefix,
			DXContinent:      spot.DXContinent,
			FrequencyHz:      spot.FrequencyHz,
			Band:             spot.Band,
			Mode:             spot.Mode,
			SignalDB:         spot.SignalDB,
			SpeedWPM:         spot.SpeedWPM,
			TransmitMode:     spot.TransmitMode,
			Source:           spot.Source,
		},
	}
}
