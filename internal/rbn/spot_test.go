package rbn

import (
	"testing"
	"time"
)

func TestDenseArchiveSpotIDUsesDayLocalContiguousRange(t *testing.T) {
	day := time.Date(2026, 8, 21, 12, 34, 56, 0, time.UTC)

	first := DenseArchiveSpotID(day, 0)
	second := DenseArchiveSpotID(day, 1)
	nextDay := DenseArchiveSpotID(day.Add(24*time.Hour), 0)

	if second != first+1 {
		t.Fatalf("second id = %d, want %d", second, first+1)
	}
	if nextDay-first != archiveSpotIDDayStride {
		t.Fatalf("next day distance = %d, want %d", nextDay-first, archiveSpotIDDayStride)
	}
}

func TestUTCSpotBucketKeys(t *testing.T) {
	ts := time.Date(2025, 11, 30, 23, 59, 58, 0, time.FixedZone("test", -6*60*60))

	if got, want := DayKeyUTC(ts), 20251201; got != want {
		t.Fatalf("day key = %d, want %d", got, want)
	}
	if got, want := ThreeHourBucketKeyUTC(ts), 2025120103; got != want {
		t.Fatalf("3h bucket key = %d, want %d", got, want)
	}
}

func TestUTCSpotBucketKeysFloorToThreeHourBoundary(t *testing.T) {
	for _, tc := range []struct {
		hour int
		want int
	}{
		{0, 2025113000},
		{1, 2025113000},
		{2, 2025113000},
		{3, 2025113003},
		{5, 2025113003},
		{21, 2025113021},
		{23, 2025113021},
	} {
		ts := time.Date(2025, 11, 30, tc.hour, 59, 59, 0, time.UTC)
		if got := ThreeHourBucketKeyUTC(ts); got != tc.want {
			t.Fatalf("hour %d bucket key = %d, want %d", tc.hour, got, tc.want)
		}
	}
}
