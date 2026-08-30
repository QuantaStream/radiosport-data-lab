package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCallsNormalizesDeduplicatesAndSorts(t *testing.T) {
	got := parseCalls("ti8x, 8p5a, bad call!, TI8X, v47t")
	want := []string{"8P5A", "TI8X", "V47T"}
	if len(got) != len(want) {
		t.Fatalf("parseCalls length = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseCalls[%d] = %q, want %q: %v", i, got[i], want[i], got)
		}
	}
}

func TestScanArchiveCountsTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20231022.csv")
	csv := "callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode\n" +
		"KP3CW,KP3,NA,28025.0,10m,8P5A,8P,NA,CQ,14,2023-10-22 14:00:00,28,CW\n" +
		"K3LR,K,NA,21025.0,15m,6Y1V,6Y,NA,CQ,20,2023-10-22 14:01:00,29,CW\n" +
		"K3LR,K,NA,21026.0,15m,8P5A,8P,NA,CQ,19,2023-10-22 14:02:00,29,CW\n" +
		"(3 rows)\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := scanArchive(path, []string{"6Y1V", "8P5A", "TI8X"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Day != "20231022" {
		t.Fatalf("Day = %q, want 20231022", result.Day)
	}
	if result.Rows != 3 {
		t.Fatalf("Rows = %d, want 3", result.Rows)
	}
	if result.SkippedFooter != 1 {
		t.Fatalf("SkippedFooter = %d, want 1", result.SkippedFooter)
	}
	if result.Counts["8P5A"] != 2 || result.Counts["6Y1V"] != 1 || result.Counts["TI8X"] != 0 {
		t.Fatalf("Counts = %#v", result.Counts)
	}
	if result.Total != 3 {
		t.Fatalf("Total = %d, want 3", result.Total)
	}
}
