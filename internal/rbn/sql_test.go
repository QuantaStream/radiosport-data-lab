package rbn

import (
	"testing"
	"time"
)

func TestSpotSQLArgs(t *testing.T) {
	spot := Spot{
		SpotID:           42,
		SpottedAt:        time.Date(2026, 8, 21, 0, 0, 1, 0, time.UTC),
		SpotDayKey:       20260821,
		Spot3HBucketKey:  2026082100,
		SpotterCall:      "G4IRN",
		SpotterPrefix:    "G",
		SpotterContinent: "EU",
		DXCall:           "KC2SIZ",
		DXPrefix:         "K",
		DXContinent:      "NA",
		FrequencyKHz:     14054.4,
		Band:             "20m",
		Mode:             "CQ",
		SignalDB:         25,
		SpeedWPM:         13,
		TransmitMode:     "CW",
		Source:           "telnet",
	}
	args := SpotSQLArgs(spot)
	if got, want := len(args), 17; got != want {
		t.Fatalf("arg count = %d, want %d", got, want)
	}
	if got, want := args[1], "2026-08-21 00:00:01"; got != want {
		t.Fatalf("spotted_at arg = %v, want %v", got, want)
	}
	if got, want := args[2], 20260821; got != want {
		t.Fatalf("spot_day_key arg = %v, want %v", got, want)
	}
	if got, want := args[3], 2026082100; got != want {
		t.Fatalf("spot_3h_bucket_key arg = %v, want %v", got, want)
	}
	if got, want := args[7], "KC2SIZ"; got != want {
		t.Fatalf("dx_call arg = %v, want %v", got, want)
	}
	if got, want := args[10], 14054.4; got != want {
		t.Fatalf("frequency_khz arg = %v, want %v", got, want)
	}
}
