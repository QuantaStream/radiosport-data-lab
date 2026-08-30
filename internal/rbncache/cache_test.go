package rbncache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func TestBuildArchiveCachesFocusedDXCall(t *testing.T) {
	cacheDir := t.TempDir()
	archivePath := writeArchive(t, "20251129.csv", strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"TI7W,TI,NA,7023.4,40m,TI8X,TI,NA,CQ,31,2025-11-29 00:00:36,33,CW",
		"K5TR,K,NA,21012.0,15m,N7ZG,K,NA,CQ,18,2025-11-29 00:01:00,29,CW",
		"ZF1A,ZF,NA,7024.1,40m,TI8X,TI,NA,CQ,24,2025-11-29 00:02:36,32,CW",
		"(3 rows)",
	}, "\n"))

	result, err := BuildArchive(context.Background(), archivePath, BuildOptions{
		CacheDir:     cacheDir,
		SpotType:     rbn.FlatSpotEventType,
		DXCalls:      []string{"ti8x"},
		DenseSpotIDs: true,
		Now: func() time.Time {
			return time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Manifest.Rows, 3; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	if got, want := result.Manifest.Emitted, 2; got != want {
		t.Fatalf("emitted = %d, want %d", got, want)
	}
	if got, want := result.Manifest.SkippedFooter, 1; got != want {
		t.Fatalf("skipped_footer = %d, want %d", got, want)
	}
	if got, want := len(result.Manifest.DXCalls), 1; got != want {
		t.Fatalf("manifest calls = %d, want %d", got, want)
	}
	if got, want := result.Manifest.DXCalls[0].DXCall, "TI8X"; got != want {
		t.Fatalf("manifest dx_call = %q, want %q", got, want)
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("manifest was not written: %v", err)
	}

	spots, stats, err := ReadCallSpots(context.Background(), cacheDir, time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC), "TI8X")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := stats.Records, 2; got != want {
		t.Fatalf("records = %d, want %d", got, want)
	}
	if got, want := len(spots), 2; got != want {
		t.Fatalf("spots = %d, want %d", got, want)
	}
	if got, want := spots[0].SpotID, rbn.DenseArchiveSpotID(spots[0].SpottedAt, 0); got != want {
		t.Fatalf("spot_id[0] = %d, want %d", got, want)
	}
	if got, want := spots[1].SpotID, rbn.DenseArchiveSpotID(spots[1].SpottedAt, 1); got != want {
		t.Fatalf("spot_id[1] = %d, want %d", got, want)
	}
	if got, want := spots[0].Activity5MKey, "TI8X|40M|CW|202511290000"; got != want {
		t.Fatalf("activity key = %q, want %q", got, want)
	}
}

func TestBuildArchiveEscapesSlashCallsignCacheFile(t *testing.T) {
	cacheDir := t.TempDir()
	archivePath := writeArchive(t, "20260821.csv", strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"DK9IP,DL,EU,14044,20m,OE/DL7UZO/P,OE,EU,CQ,9,2026-08-21 11:48:29,30,CW",
	}, "\n"))

	_, err := BuildArchive(context.Background(), archivePath, BuildOptions{
		CacheDir: cacheDir,
		DXCalls:  []string{"OE/DL7UZO/P"},
	})
	if err != nil {
		t.Fatal(err)
	}
	path, _, err := CallPath(cacheDir, time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), "OE/DL7UZO/P")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("escaped callsign file was not written: %v", err)
	}
	if strings.Contains(filepath.Base(path), "/") {
		t.Fatalf("cache filename contains slash: %s", path)
	}
}

func TestBuildArchiveWritesEmptyFileForFocusedCallWithNoSpots(t *testing.T) {
	cacheDir := t.TempDir()
	archivePath := writeArchive(t, "20251129.csv", strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"TI7W,TI,NA,7023.4,40m,N7ZG,K,NA,CQ,31,2025-11-29 00:00:36,33,CW",
	}, "\n"))

	result, err := BuildArchive(context.Background(), archivePath, BuildOptions{
		CacheDir: cacheDir,
		DXCalls:  []string{"TI8X"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := result.Manifest.Emitted, 0; got != want {
		t.Fatalf("emitted = %d, want %d", got, want)
	}
	if got, want := result.Manifest.DXCalls[0].Spots, 0; got != want {
		t.Fatalf("manifest spots = %d, want %d", got, want)
	}
	spots, stats, err := ReadCallSpots(context.Background(), cacheDir, time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC), "TI8X")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(spots), 0; got != want {
		t.Fatalf("spots = %d, want %d", got, want)
	}
	if got, want := stats.Records, 0; got != want {
		t.Fatalf("records = %d, want %d", got, want)
	}
}

func TestReadCallSpotsMissingFocusedFileReturnsEmpty(t *testing.T) {
	cacheDir := t.TempDir()
	day := time.Date(2023, 11, 5, 0, 0, 0, 0, time.UTC)

	spots, stats, err := ReadCallSpots(context.Background(), cacheDir, day, "6Y1V")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(spots), 0; got != want {
		t.Fatalf("spots = %d, want %d", got, want)
	}
	if got, want := stats.DXCall, "6Y1V"; got != want {
		t.Fatalf("DXCall = %q, want %q", got, want)
	}
	if got, want := stats.Records, 0; got != want {
		t.Fatalf("records = %d, want %d", got, want)
	}
	if !strings.HasSuffix(stats.Path, "6Y1V.jsonl") {
		t.Fatalf("stats path does not point to focused cache file: %s", stats.Path)
	}
}

func TestBuildArchiveRequiresFocusedCallsign(t *testing.T) {
	archivePath := writeArchive(t, "20251129.csv", strings.Join([]string{
		"callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode",
		"TI7W,TI,NA,7023.4,40m,TI8X,TI,NA,CQ,31,2025-11-29 00:00:36,33,CW",
	}, "\n"))

	_, err := BuildArchive(context.Background(), archivePath, BuildOptions{CacheDir: t.TempDir()})
	if err == nil {
		t.Fatal("BuildArchive succeeded without a DX callsign filter")
	}
}

func TestParseCacheDay(t *testing.T) {
	for _, input := range []string{"2025-11-29", "20251129"} {
		day, err := ParseCacheDay(input)
		if err != nil {
			t.Fatalf("ParseCacheDay(%q): %v", input, err)
		}
		if got, want := day.Format("2006-01-02"), "2025-11-29"; got != want {
			t.Fatalf("day = %q, want %q", got, want)
		}
	}
	if _, err := ParseCacheDay("11/29/2025"); err == nil {
		t.Fatal("ParseCacheDay accepted unsupported date")
	}
}

func writeArchive(t *testing.T, name string, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
