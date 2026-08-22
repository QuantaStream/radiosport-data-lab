package rbn

import (
	"testing"
	"time"
)

func TestSpotSQLArgs(t *testing.T) {
	spot := Spot{
		SpotID:           42,
		SpottedAt:        time.Date(2026, 8, 21, 0, 0, 1, 0, time.UTC),
		SpotterCall:      "G4IRN",
		SpotterPrefix:    "G",
		SpotterContinent: "EU",
		DXCall:           "KC2SIZ",
		DXPrefix:         "K",
		DXContinent:      "NA",
		FrequencyHz:      14054400,
		Band:             "20m",
		Mode:             "CQ",
		SignalDB:         25,
		SpeedWPM:         13,
		TransmitMode:     "CW",
		Source:           "telnet",
	}
	args := SpotSQLArgs(spot)
	if got, want := len(args), 15; got != want {
		t.Fatalf("arg count = %d, want %d", got, want)
	}
	if got, want := args[1], "2026-08-21 00:00:01"; got != want {
		t.Fatalf("spotted_at arg = %v, want %v", got, want)
	}
	if got, want := args[5], "KC2SIZ"; got != want {
		t.Fatalf("dx_call arg = %v, want %v", got, want)
	}
}
