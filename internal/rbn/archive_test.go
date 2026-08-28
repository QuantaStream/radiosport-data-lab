package rbn

import (
	"strings"
	"testing"
	"time"
)

func TestReadArchiveCSVSkipsFooter(t *testing.T) {
	input := strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"G4IRN,G,EU,14054.4,20m,KC2SIZ,K,NA,CQ,25,2026-08-21 00:00:00,13,CW",
		"KM3T-5,K,NA,21150,15m,CS3B,CT3,AF,NCDXF B,14,2026-08-21 00:00:04,23,CW",
		"(2 rows)",
	}, "\n")

	var spots []Spot
	stats, err := ReadArchiveCSV(strings.NewReader(input), func(spot Spot) error {
		spots = append(spots, spot)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rows != 2 || stats.SkippedFooter != 1 || stats.RejectedRows != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if got, want := len(spots), 2; got != want {
		t.Fatalf("spots = %d, want %d", got, want)
	}
	if got, want := spots[0].SpotterCall, "G4IRN"; got != want {
		t.Fatalf("spotter = %q, want %q", got, want)
	}
	if got, want := spots[0].DXCall, "KC2SIZ"; got != want {
		t.Fatalf("dx = %q, want %q", got, want)
	}
	if got, want := spots[0].FrequencyKHz, 14054.4; got != want {
		t.Fatalf("frequency_khz = %.1f, want %.1f", got, want)
	}
	if got, want := spots[0].SpottedAt, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("spotted_at = %s, want %s", got, want)
	}
	if got, want := spots[1].SpotDayKey, 20260821; got != want {
		t.Fatalf("spot_day_key = %d, want %d", got, want)
	}
	if got, want := spots[1].Spot3HBucketKey, 2026082100; got != want {
		t.Fatalf("spot_3h_bucket_key = %d, want %d", got, want)
	}
	if got, want := spots[1].Spot5MBucketKey, 202608210000; got != want {
		t.Fatalf("spot_5m_bucket_key = %d, want %d", got, want)
	}
	if got, want := spots[1].Activity5MKey, "CS3B|15M|CW|202608210000"; got != want {
		t.Fatalf("activity_5m_key = %q, want %q", got, want)
	}
}

func TestReadArchiveCSVUsesFilenameDateFallbackShape(t *testing.T) {
	input := strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"G4IRN,G,EU,14054.4,20m,KC2SIZ,K,NA,CQ,25,0000Z,13,CW",
	}, "\n")
	archiveDate := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)

	var spots []Spot
	stats, err := ReadArchiveCSVWithDate(strings.NewReader(input), archiveDate, func(spot Spot) error {
		spots = append(spots, spot)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Rows != 1 || stats.RejectedRows != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	want := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	if got := spots[0].SpottedAt; !got.Equal(want) {
		t.Fatalf("spotted_at = %s, want %s", got, want)
	}
	if got, want := spots[0].SpotDayKey, 20260821; got != want {
		t.Fatalf("spot_day_key = %d, want %d", got, want)
	}
	if got, want := spots[0].Spot3HBucketKey, 2026082100; got != want {
		t.Fatalf("spot_3h_bucket_key = %d, want %d", got, want)
	}
	if got, want := spots[0].Spot5MBucketKey, 202608210000; got != want {
		t.Fatalf("spot_5m_bucket_key = %d, want %d", got, want)
	}
}
