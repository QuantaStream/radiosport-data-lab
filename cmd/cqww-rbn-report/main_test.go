package main

import "testing"

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
