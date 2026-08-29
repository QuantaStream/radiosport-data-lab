package stationactivity

import (
	"math"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func TestBuildSummariesEmitsAllAndContinentRows(t *testing.T) {
	base := time.Date(2025, 11, 29, 1, 2, 0, 0, time.UTC)
	key := rbn.Activity5MKey("TI8X", "20M", "CW", base)
	activityID := rbn.Activity5MID(key)
	observations := []Observation{
		{DXCall: "ti8x", DXPrefix: "TI", DXContinent: "NA", Activity5MID: activityID, Activity5MKey: key, SpottedAt: base, Band: "20m", Mode: "CW", SpotterCall: "K3LR", SpotterPrefix: "K", SpotterContinent: "NA", SignalDB: 10},
		{DXCall: "TI8X", DXPrefix: "TI", DXContinent: "NA", Activity5MID: activityID, Activity5MKey: key, SpottedAt: base.Add(time.Minute), Band: "20m", Mode: "CW", SpotterCall: "W6YX", SpotterPrefix: "K", SpotterContinent: "NA", SignalDB: 20},
		{DXCall: "TI8X", DXPrefix: "TI", DXContinent: "NA", Activity5MID: activityID, Activity5MKey: key, SpottedAt: base.Add(2 * time.Minute), Band: "20m", Mode: "CW", SpotterCall: "DF2CK", SpotterPrefix: "DL", SpotterContinent: "EU", SignalDB: 30},
	}

	summaries := BuildSummaries(observations, Options{SourceTable: "spots_flat", ComputedAt: base, EmitAllContinent: true})

	if len(summaries) != 3 {
		t.Fatalf("len(summaries) = %d, want 3", len(summaries))
	}
	byContinent := map[string]Summary{}
	for _, summary := range summaries {
		byContinent[summary.SpotterContinent] = summary
		if err := ValidateSummary(summary); err != nil {
			t.Fatalf("ValidateSummary(%+v): %v", summary, err)
		}
	}
	all := byContinent[AllContinents]
	if all.SpotCount != 3 || all.DistinctSpotters != 3 || all.DistinctSpotterPrefixes != 2 {
		t.Fatalf("unexpected ALL summary: %+v", all)
	}
	if all.AvgSignalDB != 20 || all.P50SignalDB != 20 || all.P90SignalDB != 30 {
		t.Fatalf("unexpected signal stats: avg=%v p50=%v p90=%v", all.AvgSignalDB, all.P50SignalDB, all.P90SignalDB)
	}
	wantReach := 3 * math.Log1p(2) * 20
	if math.Abs(all.ReachScore-wantReach) > 0.0000001 {
		t.Fatalf("reach score = %v, want %v", all.ReachScore, wantReach)
	}
	if byContinent["NA"].SpotCount != 2 || byContinent["EU"].SpotCount != 1 {
		t.Fatalf("unexpected continent summaries: %+v", byContinent)
	}
}
