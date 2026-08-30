package spotterprofile

import (
	"math"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/geo"
)

func TestBuildSummariesComputesVolumeWeightAndPercentiles(t *testing.T) {
	base := time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC)
	observations := []Observation{
		{SpotterCall: "k3lr", SpotterPrefix: "K", SpotterContinent: "NA", SpotterCountry: "United States", SpotterCQZone: 5, SpotterITUZone: 8, SpotterLatitude: 37.6, SpotterLongitude: -91.87, SpotterGeoSource: geo.SourceCTY, SpotterGeoConfidence: geo.ConfidenceCountryCentroid, DXCall: "TI8X", DXPrefix: "TI", SignalDB: 10, SpottedAt: base},
		{SpotterCall: "K3LR", SpotterPrefix: "K", SpotterContinent: "NA", SpotterCountry: "United States", SpotterCQZone: 5, SpotterITUZone: 8, SpotterLatitude: 37.6, SpotterLongitude: -91.87, SpotterGeoSource: geo.SourceCTY, SpotterGeoConfidence: geo.ConfidenceCountryCentroid, DXCall: "TI8X", DXPrefix: "TI", SignalDB: 20, SpottedAt: base.Add(1 * time.Hour)},
		{SpotterCall: "K3LR", SpotterPrefix: "K", SpotterContinent: "NA", SpotterCountry: "United States", SpotterCQZone: 5, SpotterITUZone: 8, SpotterLatitude: 37.6, SpotterLongitude: -91.87, SpotterGeoSource: geo.SourceCTY, SpotterGeoConfidence: geo.ConfidenceCountryCentroid, DXCall: "N7ZG", DXPrefix: "K", SignalDB: 100, SpottedAt: base.Add(2 * time.Hour)},
		{SpotterCall: "DF2CK", SpotterPrefix: "DL", SpotterContinent: "EU", SpotterCountry: "Fed. Rep. of Germany", SpotterCQZone: 14, SpotterITUZone: 28, SpotterLatitude: 51, SpotterLongitude: 10, SpotterGeoSource: geo.SourceCTY, SpotterGeoConfidence: geo.ConfidenceCountryCentroid, DXCall: "TI8X", DXPrefix: "TI", SignalDB: 40, SpottedAt: base},
	}

	summaries := BuildSummaries(observations, Options{
		ProfileKind: "contest",
		WindowStart: base,
		WindowEnd:   base.Add(48 * time.Hour),
		ComputedAt:  base.Add(72 * time.Hour),
	})

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	k3lr := summaries[1]
	if k3lr.SpotterCall != "K3LR" {
		t.Fatalf("second summary = %s, want K3LR", k3lr.SpotterCall)
	}
	if k3lr.TotalSpots != 3 || k3lr.ActiveHours != 3 || k3lr.DistinctDXCalls != 2 || k3lr.DistinctDXPrefixes != 2 {
		t.Fatalf("unexpected K3LR counters: %+v", k3lr)
	}
	if k3lr.CountryName != "United States" || k3lr.CQZone != 5 || k3lr.ITUZone != 8 {
		t.Fatalf("unexpected K3LR country fields: %+v", k3lr)
	}
	if k3lr.Latitude != 37.6 || k3lr.Longitude != -91.87 {
		t.Fatalf("unexpected K3LR geo = %v/%v", k3lr.Latitude, k3lr.Longitude)
	}
	if k3lr.GeoSource != geo.SourceCTY || k3lr.GeoConfidence != geo.ConfidenceCountryCentroid {
		t.Fatalf("unexpected K3LR geo provenance: %s/%s", k3lr.GeoSource, k3lr.GeoConfidence)
	}
	nodeEvent := NewNodeEvent(k3lr)
	if nodeEvent.Data.Latitude != 37.6 || nodeEvent.Data.Longitude != -91.87 || nodeEvent.Data.GeoSource != geo.SourceCTY {
		t.Fatalf("node event geo = %+v", nodeEvent.Data)
	}
	profileEvent := NewProfileEvent(k3lr)
	if profileEvent.Data.CountryName != "United States" || profileEvent.Data.GeoConfidence != geo.ConfidenceCountryCentroid {
		t.Fatalf("profile event geo = %+v", profileEvent.Data)
	}
	if math.Abs(k3lr.AvgSignalDB-43.333333333333336) > 0.0000001 || k3lr.P50SignalDB != 20 || k3lr.P90SignalDB != 100 || k3lr.NormalizationOffsetDB != 20 {
		t.Fatalf("unexpected K3LR signal stats: avg=%v p50=%v p90=%v", k3lr.AvgSignalDB, k3lr.P50SignalDB, k3lr.P90SignalDB)
	}
	wantWeight := 1 / math.Sqrt(3)
	if math.Abs(k3lr.SpotterWeight-wantWeight) > 0.0000001 {
		t.Fatalf("spotter weight = %v, want %v", k3lr.SpotterWeight, wantWeight)
	}
	if k3lr.ProfileQuality != "sparse" {
		t.Fatalf("profile quality = %s, want sparse", k3lr.ProfileQuality)
	}
}
