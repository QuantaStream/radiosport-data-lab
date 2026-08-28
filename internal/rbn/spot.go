package rbn

import (
	"hash/fnv"
	"strconv"
	"strings"
	"time"
)

const SourceArchive = "archive"

const UnknownValue = "UNKNOWN"

const archiveSpotIDDayStride uint64 = 10000000

// Spot is the normalized event shape used by both archive and telnet ingestion.
type Spot struct {
	SpotID           uint64
	SpottedAt        time.Time
	SpotDayKey       int
	Spot3HBucketKey  int
	SpotterCall      string
	SpotterPrefix    string
	SpotterContinent string
	DXCall           string
	DXPrefix         string
	DXContinent      string
	FrequencyKHz     float64
	Band             string
	Mode             string
	SignalDB         int
	SpeedWPM         int
	TransmitMode     string
	Source           string
}

type CallsignInfo struct {
	Prefix    string
	Continent string
}

type CallsignLookup interface {
	LookupCallsign(call string) (CallsignInfo, bool)
}

func NormalizeCallsign(input string) (string, bool) {
	call := strings.ToUpper(strings.TrimSpace(input))
	if len(call) < 3 || len(call) > 16 {
		return "", false
	}
	for _, r := range call {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '/', r == '-':
		default:
			return "", false
		}
	}
	return call, true
}

func StableSpotID(s Spot) uint64 {
	h := fnv.New64a()
	writePart := func(v string) {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	writePart(s.SpottedAt.UTC().Format(time.RFC3339Nano))
	writePart(s.SpotterCall)
	writePart(s.DXCall)
	writePart(formatFrequencyKHz(s.FrequencyKHz))
	writePart(s.Mode)
	writePart(strconv.Itoa(s.SignalDB))
	writePart(strconv.Itoa(s.SpeedWPM))
	writePart(s.TransmitMode)
	// Keep the synthetic key inside signed-int64 range for SQL driver portability.
	return h.Sum64() & ((uint64(1) << 63) - 1)
}

func DenseArchiveSpotID(spottedAt time.Time, zeroBasedOrdinal int) uint64 {
	if zeroBasedOrdinal < 0 {
		zeroBasedOrdinal = 0
	}
	unixDay := spottedAt.UTC().Unix() / int64((24 * time.Hour).Seconds())
	if unixDay < 0 {
		unixDay = 0
	}
	return uint64(unixDay)*archiveSpotIDDayStride + uint64(zeroBasedOrdinal) + 1
}

func DayKeyUTC(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	utc := t.UTC()
	return utc.Year()*10000 + int(utc.Month())*100 + utc.Day()
}

func ThreeHourBucketKeyUTC(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	utc := t.UTC()
	bucketHour := (utc.Hour() / 3) * 3
	return DayKeyUTC(utc)*100 + bucketHour
}

func populateSpotTimeKeys(spot *Spot) {
	spot.SpotDayKey = DayKeyUTC(spot.SpottedAt)
	spot.Spot3HBucketKey = ThreeHourBucketKeyUTC(spot.SpottedAt)
}

func formatFrequencyKHz(freq float64) string {
	return strconv.FormatFloat(freq, 'f', 1, 64)
}
