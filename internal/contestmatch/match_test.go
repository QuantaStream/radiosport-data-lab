package contestmatch

import (
	"math"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func TestMatchQSOsToSpotsUsesWindowAndFrequencyTolerance(t *testing.T) {
	qsoAt := time.Date(2025, 11, 29, 0, 4, 30, 0, time.UTC)
	qso := testQSO(qsoAt, 7050)
	spots := []rbn.Spot{
		testSpot(10, qsoAt.Add(-20*time.Second), 7050.2, "TI8X", "40m", "CW"),
		testSpot(11, qsoAt.Add(6*time.Minute), 7050.1, "TI8X", "40m", "CW"),
		testSpot(12, qsoAt.Add(10*time.Second), 7055.2, "TI8X", "40m", "CW"),
		testSpot(13, qsoAt.Add(5*time.Second), 7050.1, "K0DU", "40m", "CW"),
	}

	matches := MatchQSOsToSpots([]cabrillo.QSO{qso}, spots, Options{
		Window:                5 * time.Minute,
		FrequencyToleranceKHz: 1.0,
		DenseMatchIDs:         true,
		LoadedAt:              time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
	})

	if got, want := len(matches), 1; got != want {
		t.Fatalf("len(matches) = %d, want %d", got, want)
	}
	if matches[0].MatchID != 1 {
		t.Fatalf("dense match id = %d, want 1", matches[0].MatchID)
	}
	if matches[0].SpotID != 10 || matches[0].QSOID != qso.QSOID {
		t.Fatalf("match refs = qso %d spot %d", matches[0].QSOID, matches[0].SpotID)
	}
	if matches[0].TimeDeltaSeconds != -20 || matches[0].AbsTimeDeltaSeconds != 20 {
		t.Fatalf("time deltas = %d/%d", matches[0].TimeDeltaSeconds, matches[0].AbsTimeDeltaSeconds)
	}
	if math.Abs(matches[0].FrequencyDeltaKHz-0.2) > 0.001 || math.Abs(matches[0].AbsFrequencyDeltaKHz-0.2) > 0.001 {
		t.Fatalf("frequency deltas = %.3f/%.3f", matches[0].FrequencyDeltaKHz, matches[0].AbsFrequencyDeltaKHz)
	}
	if matches[0].MatchRank != 1 || matches[0].IsBestMatch != 1 {
		t.Fatalf("rank/best = %d/%d", matches[0].MatchRank, matches[0].IsBestMatch)
	}
	if matches[0].TimeScore <= 0 || matches[0].FrequencyScore <= 0 || matches[0].MatchScore <= 0 {
		t.Fatalf("scores were not populated: time=%.3f frequency=%.3f match=%.3f", matches[0].TimeScore, matches[0].FrequencyScore, matches[0].MatchScore)
	}
}

func TestMatchQSOsToSpotsCapsClosestMatchesPerQSO(t *testing.T) {
	qsoAt := time.Date(2025, 11, 29, 0, 5, 0, 0, time.UTC)
	qso := testQSO(qsoAt, 14025)
	spots := []rbn.Spot{
		testSpot(3, qsoAt.Add(40*time.Second), 14025, "TI8X", "20m", "CW"),
		testSpot(2, qsoAt.Add(-5*time.Second), 14025.1, "TI8X", "20m", "CW"),
		testSpot(1, qsoAt.Add(20*time.Second), 14024.9, "TI8X", "20m", "CW"),
	}

	matches := MatchQSOsToSpots([]cabrillo.QSO{qso}, spots, Options{
		Window:           time.Minute,
		MaxMatchesPerQSO: 2,
		DenseMatchIDs:    true,
	})

	if got, want := len(matches), 2; got != want {
		t.Fatalf("len(matches) = %d, want %d", got, want)
	}
	if matches[0].SpotID != 2 || matches[1].SpotID != 1 {
		t.Fatalf("closest spots = %d, %d", matches[0].SpotID, matches[1].SpotID)
	}
	if matches[0].MatchRank != 1 || matches[0].IsBestMatch != 1 {
		t.Fatalf("first rank/best = %d/%d", matches[0].MatchRank, matches[0].IsBestMatch)
	}
	if matches[1].MatchRank != 2 || matches[1].IsBestMatch != 0 {
		t.Fatalf("second rank/best = %d/%d", matches[1].MatchRank, matches[1].IsBestMatch)
	}
}

func TestMatchQSOsToSpotsUsesSignalAsFinalRankingTieBreaker(t *testing.T) {
	qsoAt := time.Date(2025, 11, 29, 0, 5, 0, 0, time.UTC)
	qso := testQSO(qsoAt, 14025)
	weak := testSpot(1, qsoAt.Add(15*time.Second), 14025.1, "TI8X", "20m", "CW")
	weak.SignalDB = 9
	strong := testSpot(2, qsoAt.Add(15*time.Second), 14025.1, "TI8X", "20m", "CW")
	strong.SignalDB = 31

	matches := MatchQSOsToSpots([]cabrillo.QSO{qso}, []rbn.Spot{weak, strong}, Options{
		Window:        time.Minute,
		DenseMatchIDs: true,
	})

	if got, want := len(matches), 2; got != want {
		t.Fatalf("len(matches) = %d, want %d", got, want)
	}
	if got, want := matches[0].SpotID, uint64(2); got != want {
		t.Fatalf("best spot id = %d, want %d", got, want)
	}
	if matches[0].MatchRank != 1 || matches[0].IsBestMatch != 1 {
		t.Fatalf("best rank flags = %d/%d", matches[0].MatchRank, matches[0].IsBestMatch)
	}
	if matches[1].MatchRank != 2 || matches[1].IsBestMatch != 0 {
		t.Fatalf("second rank flags = %d/%d", matches[1].MatchRank, matches[1].IsBestMatch)
	}
}

func TestNewEventFormatsPayload(t *testing.T) {
	qsoAt := time.Date(2025, 11, 29, 0, 5, 0, 0, time.UTC)
	match := NewMatch(
		testQSO(qsoAt, 14025),
		testSpot(99, qsoAt.Add(30*time.Second), 14025.4, "TI8X", "20m", "CW"),
		5*time.Minute,
		1.0,
		time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC),
	)

	event := NewEvent(match)
	if event.Type != EventType {
		t.Fatalf("event type = %q", event.Type)
	}
	if event.Data.QSOAt != "2025-11-29T00:05:00Z" || event.Data.SpottedAt != "2025-11-29T00:05:30Z" {
		t.Fatalf("times = %q/%q", event.Data.QSOAt, event.Data.SpottedAt)
	}
	if event.Data.MatchKind != KindSameBucket || event.Data.SameActivityBucket != 1 {
		t.Fatalf("kind/bucket = %q/%d", event.Data.MatchKind, event.Data.SameActivityBucket)
	}
	if math.Abs(event.Data.TimeScore-90.0) > 0.001 {
		t.Fatalf("time_score = %.3f, want 90.000", event.Data.TimeScore)
	}
	if math.Abs(event.Data.FrequencyScore-60.0) > 0.001 {
		t.Fatalf("frequency_score = %.3f, want 60.000", event.Data.FrequencyScore)
	}
	if math.Abs(event.Data.MatchScore-83.2) > 0.001 {
		t.Fatalf("match_score = %.3f, want 83.200", event.Data.MatchScore)
	}
}

func testQSO(qsoAt time.Time, freq float64) cabrillo.QSO {
	qso := cabrillo.QSO{
		QSOID:            9001,
		LogID:            "cq-ww-cw-2025:TI8X",
		ContestID:        "cq-ww-cw-2025",
		QSOAt:            qsoAt,
		QSODayKey:        rbn.DayKeyUTC(qsoAt),
		QSO3HBucketKey:   rbn.ThreeHourBucketKeyUTC(qsoAt),
		QSO5MBucketKey:   rbn.FiveMinuteBucketKeyUTC(qsoAt),
		StationCall:      "TI8X",
		StationPrefix:    "TI",
		StationContinent: "NA",
		WorkedCall:       "K0DU",
		WorkedPrefix:     "K",
		WorkedContinent:  "NA",
		FrequencyKHz:     freq,
		Band:             "40m",
		Mode:             "CW",
		SentExchange:     "7",
		ReceivedExchange: "04",
		SourceFile:       "ti8x.log",
	}
	if freq >= 14000 {
		qso.Band = "20m"
	}
	qso.Activity5MKey = rbn.Activity5MKey(qso.StationCall, qso.Band, qso.Mode, qso.QSOAt)
	qso.Activity5MID = rbn.Activity5MID(qso.Activity5MKey)
	return qso
}

func testSpot(id uint64, spottedAt time.Time, freq float64, dxCall string, band string, mode string) rbn.Spot {
	spot := rbn.Spot{
		SpotID:           id,
		SpottedAt:        spottedAt,
		SpotterCall:      "K5TR",
		SpotterPrefix:    "K",
		SpotterContinent: "NA",
		DXCall:           dxCall,
		DXPrefix:         "TI",
		DXContinent:      "NA",
		FrequencyKHz:     freq,
		Band:             band,
		Mode:             "CQ",
		SignalDB:         22,
		SpeedWPM:         33,
		TransmitMode:     mode,
		Source:           rbn.SourceArchive,
	}
	spot.SpotDayKey = rbn.DayKeyUTC(spot.SpottedAt)
	spot.Spot3HBucketKey = rbn.ThreeHourBucketKeyUTC(spot.SpottedAt)
	spot.Spot5MBucketKey = rbn.FiveMinuteBucketKeyUTC(spot.SpottedAt)
	spot.Activity5MKey = rbn.Activity5MKey(spot.DXCall, spot.Band, mode, spot.SpottedAt)
	spot.Activity5MID = rbn.Activity5MID(spot.Activity5MKey)
	return spot
}
