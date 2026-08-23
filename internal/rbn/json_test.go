package rbn

import (
	"testing"
	"time"
)

func TestNewSpotEventUsesDefaultType(t *testing.T) {
	event := NewSpotEvent(testSpot())
	if got, want := event.Type, DefaultSpotEventType; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
}

func TestNewSpotEventWithTypeOverridesType(t *testing.T) {
	event := NewSpotEventWithType(testSpot(), FlatSpotEventType)
	if got, want := event.Type, FlatSpotEventType; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
}

func TestNewSpotEventWithTypeFallsBackOnEmptyType(t *testing.T) {
	event := NewSpotEventWithType(testSpot(), "")
	if got, want := event.Type, DefaultSpotEventType; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
}

func testSpot() Spot {
	return Spot{
		SpotID:           1,
		SpottedAt:        time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
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
		Source:           SourceArchive,
	}
}
