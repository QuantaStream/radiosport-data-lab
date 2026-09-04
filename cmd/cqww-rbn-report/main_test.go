package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPercentile(t *testing.T) {
	tests := []struct {
		values []int
		q      float64
		want   float64
	}{
		{[]int{1}, 0.5, 1},
		{[]int{4, 1, 3, 2}, 0.5, 2.5},
		{[]int{0, 10}, 0.9, 9},
	}
	for _, test := range tests {
		if got := percentile(test.values, test.q); got != test.want {
			t.Fatalf("percentile(%v, %v) = %v, want %v", test.values, test.q, got, test.want)
		}
	}
}

func TestParseCalls(t *testing.T) {
	got := parseCalls(" ef8r,CQ9A,ef8r,bad call ")
	if len(got) != 2 || got[0] != "EF8R" || got[1] != "CQ9A" {
		t.Fatalf("parseCalls returned %v", got)
	}
}

func TestReadHighBandQSOs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "band-activity.csv")
	data := "bucket_start,station,band,qsos\n2025-11-29T07:00:00Z,EF8R,15m,10\n2025-11-29T07:05:00Z,EF8R,10m,12\n2025-11-29T07:10:00Z,EF8R,20m,99\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readHighBandQSOs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got[dayStation{"2025-11-29", "EF8R"}] != 22 {
		t.Fatalf("high-band total = %d, want 22", got[dayStation{"2025-11-29", "EF8R"}])
	}
}
