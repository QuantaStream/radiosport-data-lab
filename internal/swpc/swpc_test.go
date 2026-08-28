package swpc

import (
	"strings"
	"testing"
	"time"
)

const solarSample = `
:Product: Daily Solar Data            DSD.txt
#  Date     10.7cm Number  Hemis. Regions Field  Flux   C  M  X  S  1  2  3
2025 11 29  142     91      300      1    -999      *   5  0  0  2  0  0  0
2025 11 30  145     94      320      0    -999      *   3  0  0  1  0  0  0
`

const geomagSample = `
:Product: Daily Geomagnetic Data          DGD.txt
#  Date        A     K-indices        A     K-indices        A     K-indices
2025 11 29     7  1 1 0 2 3 3 2 1     4  1 0 0 2 2 1 0 0     5   1.00  1.33  0.67  2.00  1.67  1.67  1.33  1.00
2025 11 30     9  2 2 1 3 3 2 2 1     5  1 1 1 2 2 2 1 1     8   2.00  2.33  1.67  3.00  2.67  2.00  1.67  1.33
`

func TestParseDailySolarIndices(t *testing.T) {
	rows, err := ParseDailySolarIndices(strings.NewReader(solarSample))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := DayKeyUTC(rows[0].ObservedDate), 20251129; got != want {
		t.Fatalf("day key = %d, want %d", got, want)
	}
	if got, want := rows[1].SFI, 145.0; got != want {
		t.Fatalf("sfi = %.1f, want %.1f", got, want)
	}
	if got, want := rows[1].SunspotNumber, 94; got != want {
		t.Fatalf("sunspot number = %d, want %d", got, want)
	}
}

func TestParseDailyGeomagneticIndices(t *testing.T) {
	rows, err := ParseDailyGeomagneticIndices(strings.NewReader(geomagSample))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(rows), 2; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := rows[0].AIndex, 7; got != want {
		t.Fatalf("a index = %d, want %d", got, want)
	}
	if got, want := rows[0].APIndex, 5; got != want {
		t.Fatalf("ap index = %d, want %d", got, want)
	}
	if got, want := rows[0].KP[3], 2.00; got != want {
		t.Fatalf("kp[3] = %.2f, want %.2f", got, want)
	}
}

func TestBuildEventsForDateRange(t *testing.T) {
	solarRows, err := ParseDailySolarIndices(strings.NewReader(solarSample))
	if err != nil {
		t.Fatal(err)
	}
	geomagRows, err := ParseDailyGeomagneticIndices(strings.NewReader(geomagSample))
	if err != nil {
		t.Fatal(err)
	}
	from, _ := ParseDay("2025-11-30")
	to, _ := ParseDay("2025-11-30")
	loadedAt := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)

	events, dailyCount, kCount := BuildEvents(solarRows, geomagRows, from, to, "swpc-test", loadedAt)
	if got, want := dailyCount, 1; got != want {
		t.Fatalf("daily count = %d, want %d", got, want)
	}
	if got, want := kCount, 8; got != want {
		t.Fatalf("k count = %d, want %d", got, want)
	}
	if got, want := len(events), 9; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}

	daily, ok := events[0].(DailyIndexEvent)
	if !ok {
		t.Fatalf("first event type = %T, want DailyIndexEvent", events[0])
	}
	if got, want := daily.Data.DayKey, 20251130; got != want {
		t.Fatalf("day key = %d, want %d", got, want)
	}
	if got, want := daily.Data.SFI, 145.0; got != want {
		t.Fatalf("sfi = %.1f, want %.1f", got, want)
	}

	last, ok := events[len(events)-1].(KIndex3HEvent)
	if !ok {
		t.Fatalf("last event type = %T, want KIndex3HEvent", events[len(events)-1])
	}
	if got, want := last.Data.BucketKey, 2025113021; got != want {
		t.Fatalf("bucket key = %d, want %d", got, want)
	}
	if got, want := last.Data.KPIndex, 1.33; got != want {
		t.Fatalf("kp index = %.2f, want %.2f", got, want)
	}
}

func TestParseDayAndBucketKeysUseUTC(t *testing.T) {
	ts := time.Date(2025, 11, 30, 23, 59, 0, 0, time.FixedZone("local", -6*60*60))
	if got, want := DayKeyUTC(ts), 20251201; got != want {
		t.Fatalf("day key = %d, want %d", got, want)
	}
	if got, want := ThreeHourBucketKeyUTC(ts), 2025120103; got != want {
		t.Fatalf("bucket key = %d, want %d", got, want)
	}
}
