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

func TestNewSpotEventIncludesTimeBucketKeys(t *testing.T) {
	event := NewSpotEvent(testSpot())
	if got, want := event.Data.SpotDayKey, 20260821; got != want {
		t.Fatalf("spot_day_key = %d, want %d", got, want)
	}
	if got, want := event.Data.Spot3HBucketKey, 2026082100; got != want {
		t.Fatalf("spot_3h_bucket_key = %d, want %d", got, want)
	}
	if got, want := event.Data.Spot5MBucketKey, 202608210000; got != want {
		t.Fatalf("spot_5m_bucket_key = %d, want %d", got, want)
	}
	if got, want := event.Data.Activity5MKey, "KC2SIZ|20M|CW|202608210000"; got != want {
		t.Fatalf("activity_5m_key = %q, want %q", got, want)
	}
	if event.Data.Activity5MID == 0 {
		t.Fatal("activity_5m_id = 0, want stable non-zero id")
	}
}

func TestFiveMinuteBucketKeyFloorsToBucketStart(t *testing.T) {
	ts := time.Date(2026, 8, 21, 14, 37, 22, 0, time.UTC)
	if got, want := FiveMinuteBucketKeyUTC(ts), 202608211435; got != want {
		t.Fatalf("FiveMinuteBucketKeyUTC() = %d, want %d", got, want)
	}
}

func TestFiveMinuteBucketStartFloorsToBucketStart(t *testing.T) {
	ts := time.Date(2026, 8, 21, 14, 37, 22, 0, time.UTC)
	want := time.Date(2026, 8, 21, 14, 35, 0, 0, time.UTC)
	if got := FiveMinuteBucketStartUTC(ts); !got.Equal(want) {
		t.Fatalf("FiveMinuteBucketStartUTC() = %s, want %s", got, want)
	}
}

func TestNewActivity5MBucketEvent(t *testing.T) {
	bucket := Activity5MBucketFromSpot(testSpot())
	event := NewActivity5MBucketEvent(bucket)
	if got, want := event.Type, Activity5MBucketEventType; got != want {
		t.Fatalf("type = %q, want %q", got, want)
	}
	if got, want := event.Data.Activity5MKey, "KC2SIZ|20M|CW|202608210000"; got != want {
		t.Fatalf("activity_5m_key = %q, want %q", got, want)
	}
	if got, want := event.Data.ActivityCall, "KC2SIZ"; got != want {
		t.Fatalf("activity_call = %q, want %q", got, want)
	}
	if got, want := event.Data.ActivityBand, "20M"; got != want {
		t.Fatalf("activity_band = %q, want %q", got, want)
	}
	if got, want := event.Data.ActivityMode, "CW"; got != want {
		t.Fatalf("activity_mode = %q, want %q", got, want)
	}
	if got, want := event.Data.BucketKey, 202608210000; got != want {
		t.Fatalf("bucket_key = %d, want %d", got, want)
	}
	if got, want := event.Data.BucketStart, "2026-08-21T00:00:00Z"; got != want {
		t.Fatalf("bucket_start = %q, want %q", got, want)
	}
}

func testSpot() Spot {
	return Spot{
		SpotID:           1,
		SpottedAt:        time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
		SpotDayKey:       20260821,
		Spot3HBucketKey:  2026082100,
		Spot5MBucketKey:  202608210000,
		Activity5MID:     Activity5MID("KC2SIZ|20M|CW|202608210000"),
		Activity5MKey:    "KC2SIZ|20M|CW|202608210000",
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
