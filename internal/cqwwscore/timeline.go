package cqwwscore

import (
	"fmt"
	"sort"
	"time"
)

type Bucket struct {
	BucketStart           time.Time
	BucketEnd             time.Time
	StationCall           string
	BucketQSOs            int
	BucketCountedQSOs     int
	BucketDuplicates      int
	BucketUnresolved      int
	BucketPoints          int
	BucketCountries       int
	BucketZones           int
	Bucket10M             int
	Bucket15M             int
	Bucket20M             int
	Bucket40M             int
	Bucket80M             int
	Bucket160M            int
	CumulativeQSOs        int
	CumulativeCounted     int
	CumulativeDuplicate   int
	CumulativeUnresolved  int
	CumulativePoints      int
	CumulativeCountries   int
	CumulativeZones       int
	CumulativeMultipliers int
	CumulativeScore       int64
}

type BandBucket struct {
	BucketStart  time.Time
	BucketEnd    time.Time
	StationCall  string
	Band         string
	QSOs         int
	CountedQSOs  int
	Duplicates   int
	Unresolved   int
	Points       int
	NewCountries int
	NewZones     int
}

// BuildBuckets aligns cumulative QSO states to half-open time buckets
// [BucketStart, BucketEnd). Empty buckets retain the previous cumulative state.
func BuildBuckets(states []State, start, end time.Time, interval time.Duration) ([]Bucket, []BandBucket, error) {
	if !start.Before(end) {
		return nil, nil, fmt.Errorf("start must be before end")
	}
	if interval <= 0 {
		return nil, nil, fmt.Errorf("interval must be positive")
	}
	if len(states) == 0 {
		return nil, nil, fmt.Errorf("at least one state is required")
	}
	ordered := append([]State(nil), states...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	station := ordered[0].StationCall
	var buckets []Bucket
	var bands []BandBucket
	idx := 0
	var last State
	for bucketStart := start; bucketStart.Before(end); bucketStart = bucketStart.Add(interval) {
		bucketEnd := bucketStart.Add(interval)
		if bucketEnd.After(end) {
			bucketEnd = end
		}
		b := Bucket{BucketStart: bucketStart, BucketEnd: bucketEnd, StationCall: station}
		byBand := map[string]*BandBucket{}
		for idx < len(ordered) && ordered[idx].At.Before(bucketEnd) {
			s := ordered[idx]
			last = s
			if !s.At.Before(bucketStart) {
				b.BucketQSOs++
				b.BucketCountedQSOs += s.QSOAdded
				b.BucketPoints += s.PointsAdded
				b.BucketCountries += s.CountryAdded
				b.BucketZones += s.ZoneAdded
				if s.Duplicate {
					b.BucketDuplicates++
				}
				if s.Unresolved {
					b.BucketUnresolved++
				}
				if s.QSOAdded == 1 {
					switch s.Band {
					case "10m":
						b.Bucket10M++
					case "15m":
						b.Bucket15M++
					case "20m":
						b.Bucket20M++
					case "40m":
						b.Bucket40M++
					case "80m":
						b.Bucket80M++
					case "160m":
						b.Bucket160M++
					}
				}
				bb := byBand[s.Band]
				if bb == nil {
					bb = &BandBucket{BucketStart: bucketStart, BucketEnd: bucketEnd, StationCall: station, Band: s.Band}
					byBand[s.Band] = bb
				}
				bb.QSOs++
				bb.CountedQSOs += s.QSOAdded
				bb.Points += s.PointsAdded
				bb.NewCountries += s.CountryAdded
				bb.NewZones += s.ZoneAdded
				if s.Duplicate {
					bb.Duplicates++
				}
				if s.Unresolved {
					bb.Unresolved++
				}
			}
			idx++
		}
		b.CumulativeQSOs = last.QSOCount
		b.CumulativeCounted = last.CountedQSOCount
		b.CumulativeDuplicate = last.DuplicateCount
		b.CumulativeUnresolved = last.UnresolvedCount
		b.CumulativePoints = last.QSOPoints
		b.CumulativeCountries = last.CountryMultipliers
		b.CumulativeZones = last.ZoneMultipliers
		b.CumulativeMultipliers = last.MultiplierCount
		b.CumulativeScore = last.Score
		buckets = append(buckets, b)
		keys := make([]string, 0, len(byBand))
		for band := range byBand {
			keys = append(keys, band)
		}
		sort.Strings(keys)
		for _, band := range keys {
			bands = append(bands, *byBand[band])
		}
	}
	return buckets, bands, nil
}
