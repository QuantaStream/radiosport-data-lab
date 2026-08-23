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
