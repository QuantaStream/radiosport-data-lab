package cqwwscore

import (
	"testing"
	"time"
)

func TestBuildBucketsCarriesCumulativeStateAcrossEmptyBuckets(t *testing.T) {
	start := time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC)
	states := []State{
		{At: start.Add(time.Minute), StationCall: "TEST", Band: "20m", QSOCount: 1, CountedQSOCount: 1, QSOPoints: 3, CountryMultipliers: 1, ZoneMultipliers: 1, MultiplierCount: 2, Score: 6, QSOAdded: 1, PointsAdded: 3, CountryAdded: 1, ZoneAdded: 1},
		{At: start.Add(11 * time.Minute), StationCall: "TEST", Band: "15m", QSOCount: 2, CountedQSOCount: 2, QSOPoints: 6, CountryMultipliers: 2, ZoneMultipliers: 2, MultiplierCount: 4, Score: 24, QSOAdded: 1, PointsAdded: 3, CountryAdded: 1, ZoneAdded: 1},
	}
	buckets, bands, err := BuildBuckets(states, start, start.Add(15*time.Minute), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 3 || len(bands) != 2 {
		t.Fatalf("bucket counts = %d/%d", len(buckets), len(bands))
	}
	if buckets[0].BucketQSOs != 1 || buckets[0].CumulativeScore != 6 || buckets[0].Bucket20M != 1 {
		t.Fatalf("first = %+v", buckets[0])
	}
	if buckets[1].BucketQSOs != 0 || buckets[1].CumulativeScore != 6 {
		t.Fatalf("empty = %+v", buckets[1])
	}
	if buckets[2].BucketQSOs != 1 || buckets[2].CumulativeScore != 24 || buckets[2].Bucket15M != 1 {
		t.Fatalf("last = %+v", buckets[2])
	}
}

func TestBuildBucketsDoesNotCountDuplicateInBandProduction(t *testing.T) {
	start := time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC)
	states := []State{
		{At: start, StationCall: "TEST", Band: "10m", QSOCount: 1, CountedQSOCount: 1, QSOAdded: 1},
		{At: start.Add(time.Minute), StationCall: "TEST", Band: "10m", QSOCount: 2, CountedQSOCount: 1, DuplicateCount: 1, Duplicate: true},
	}
	buckets, _, err := BuildBuckets(states, start, start.Add(5*time.Minute), 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if got := buckets[0]; got.BucketQSOs != 2 || got.BucketCountedQSOs != 1 || got.Bucket10M != 1 {
		t.Fatalf("bucket = %+v", got)
	}
}
