package rbn

import (
	"sort"
	"time"
)

const Activity5MBucketEventType = "activity_5m_bucket"

type Activity5MBucket struct {
	Activity5MID  uint64
	Activity5MKey string
	ActivityCall  string
	ActivityBand  string
	ActivityMode  string
	BucketKey     int
	BucketStart   time.Time
}

type Activity5MBucketEvent struct {
	Type string                  `json:"type"`
	Data Activity5MBucketPayload `json:"data"`
}

type Activity5MBucketPayload struct {
	Activity5MID  uint64 `json:"activity_5m_id"`
	Activity5MKey string `json:"activity_5m_key"`
	ActivityCall  string `json:"activity_call"`
	ActivityBand  string `json:"activity_band"`
	ActivityMode  string `json:"activity_mode"`
	BucketKey     int    `json:"bucket_key"`
	BucketStart   string `json:"bucket_start"`
}

func NewActivity5MBucket(call string, band string, mode string, t time.Time) Activity5MBucket {
	key := Activity5MKey(call, band, mode, t)
	return Activity5MBucket{
		Activity5MID:  Activity5MID(key),
		Activity5MKey: key,
		ActivityCall:  normalizeActivityPart(call),
		ActivityBand:  normalizeActivityPart(band),
		ActivityMode:  normalizeActivityPart(mode),
		BucketKey:     FiveMinuteBucketKeyUTC(t),
		BucketStart:   FiveMinuteBucketStartUTC(t),
	}
}

func Activity5MBucketFromSpot(spot Spot) Activity5MBucket {
	return NewActivity5MBucket(spot.DXCall, spot.Band, spotActivityMode(spot), spot.SpottedAt)
}

func NewActivity5MBucketEvent(bucket Activity5MBucket) Activity5MBucketEvent {
	return Activity5MBucketEvent{
		Type: Activity5MBucketEventType,
		Data: Activity5MBucketPayload{
			Activity5MID:  bucket.Activity5MID,
			Activity5MKey: bucket.Activity5MKey,
			ActivityCall:  bucket.ActivityCall,
			ActivityBand:  bucket.ActivityBand,
			ActivityMode:  bucket.ActivityMode,
			BucketKey:     bucket.BucketKey,
			BucketStart:   formatActivityTime(bucket.BucketStart),
		},
	}
}

func SortedActivity5MBuckets(buckets map[uint64]Activity5MBucket) []Activity5MBucket {
	ordered := make([]Activity5MBucket, 0, len(buckets))
	for _, bucket := range buckets {
		ordered = append(ordered, bucket)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Activity5MKey < ordered[j].Activity5MKey
	})
	return ordered
}

func Activity5MBucketInsertSQL() string {
	return `insert into activity_5m_buckets (
  activity_5m_id,
  activity_5m_key,
  activity_call,
  activity_band,
  activity_mode,
  bucket_key,
  bucket_start
) values (?, ?, ?, ?, ?, ?, ?)`
}

func Activity5MBucketSQLArgs(bucket Activity5MBucket) []interface{} {
	return []interface{}{
		bucket.Activity5MID,
		bucket.Activity5MKey,
		bucket.ActivityCall,
		bucket.ActivityBand,
		bucket.ActivityMode,
		bucket.BucketKey,
		formatActivitySQLTime(bucket.BucketStart),
	}
}

func FiveMinuteBucketStartUTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	utc := t.UTC()
	bucketMinute := (utc.Minute() / 5) * 5
	return time.Date(utc.Year(), utc.Month(), utc.Day(), utc.Hour(), bucketMinute, 0, 0, time.UTC)
}

func formatActivityTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatActivitySQLTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}
