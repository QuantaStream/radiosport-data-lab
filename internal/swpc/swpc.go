package swpc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	DailyIndexEventType = "swpc_daily_index"
	KIndex3HEventType   = "swpc_k_index_3h"
)

type DailySolarIndex struct {
	ObservedDate  time.Time
	SFI           float64
	SunspotNumber int
}

type DailyGeomagneticIndex struct {
	ObservedDate time.Time
	AIndex       int
	APIndex      int
	KP           [8]float64
}

type DailyIndex struct {
	DayKey        int
	ObservedDate  time.Time
	AIndex        int
	APIndex       int
	SFI           float64
	SunspotNumber int
	Source        string
	LoadedAt      time.Time
}

type KIndex3H struct {
	BucketKey   int
	BucketStart time.Time
	DayKey      int
	KIndex      int
	KPIndex     float64
	Source      string
	LoadedAt    time.Time
}

type DailyIndexEvent struct {
	Type string            `json:"type"`
	Data DailyIndexPayload `json:"data"`
}

type DailyIndexPayload struct {
	DayKey        int     `json:"day_key"`
	ObservedDate  string  `json:"observed_date"`
	AIndex        int     `json:"a_index"`
	APIndex       int     `json:"ap_index"`
	SFI           float64 `json:"sfi"`
	SunspotNumber int     `json:"sunspot_number"`
	Source        string  `json:"source"`
	LoadedAt      string  `json:"loaded_at"`
}

type KIndex3HEvent struct {
	Type string          `json:"type"`
	Data KIndex3HPayload `json:"data"`
}

type KIndex3HPayload struct {
	BucketKey   int     `json:"bucket_key"`
	BucketStart string  `json:"bucket_start"`
	DayKey      int     `json:"day_key"`
	KIndex      int     `json:"k_index"`
	KPIndex     float64 `json:"kp_index"`
	Source      string  `json:"source"`
	LoadedAt    string  `json:"loaded_at"`
}

func ParseDailySolarIndices(r io.Reader) ([]DailySolarIndex, error) {
	var rows []DailySolarIndex
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 || !looksLikeDateFields(fields[:3]) {
			continue
		}
		observedDate, err := parseDateFields(fields[:3])
		if err != nil {
			return nil, err
		}
		sfi, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			return nil, fmt.Errorf("parse solar flux on %s: %w", formatDay(observedDate), err)
		}
		sunspotNumber, err := strconv.Atoi(fields[4])
		if err != nil {
			return nil, fmt.Errorf("parse sunspot number on %s: %w", formatDay(observedDate), err)
		}
		rows = append(rows, DailySolarIndex{
			ObservedDate:  observedDate,
			SFI:           sfi,
			SunspotNumber: sunspotNumber,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func ParseDailyGeomagneticIndices(r io.Reader) ([]DailyGeomagneticIndex, error) {
	var rows []DailyGeomagneticIndex
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 30 || !looksLikeDateFields(fields[:3]) {
			continue
		}
		observedDate, err := parseDateFields(fields[:3])
		if err != nil {
			return nil, err
		}
		aIndex, err := strconv.Atoi(fields[3])
		if err != nil {
			return nil, fmt.Errorf("parse middle-latitude A index on %s: %w", formatDay(observedDate), err)
		}
		apIndex, err := strconv.Atoi(fields[21])
		if err != nil {
			return nil, fmt.Errorf("parse planetary A index on %s: %w", formatDay(observedDate), err)
		}
		var kp [8]float64
		for i := range kp {
			value, err := strconv.ParseFloat(fields[22+i], 64)
			if err != nil {
				return nil, fmt.Errorf("parse planetary Kp index %d on %s: %w", i, formatDay(observedDate), err)
			}
			kp[i] = value
		}
		rows = append(rows, DailyGeomagneticIndex{
			ObservedDate: observedDate,
			AIndex:       aIndex,
			APIndex:      apIndex,
			KP:           kp,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func BuildEvents(solar []DailySolarIndex, geomag []DailyGeomagneticIndex, from, to time.Time, source string, loadedAt time.Time) ([]interface{}, int, int) {
	solarByDay := map[int]DailySolarIndex{}
	for _, row := range solar {
		solarByDay[DayKeyUTC(row.ObservedDate)] = row
	}
	geomagByDay := map[int]DailyGeomagneticIndex{}
	for _, row := range geomag {
		geomagByDay[DayKeyUTC(row.ObservedDate)] = row
	}

	days := map[int]time.Time{}
	for _, row := range solar {
		if inRange(row.ObservedDate, from, to) {
			days[DayKeyUTC(row.ObservedDate)] = row.ObservedDate
		}
	}
	for _, row := range geomag {
		if inRange(row.ObservedDate, from, to) {
			days[DayKeyUTC(row.ObservedDate)] = row.ObservedDate
		}
	}

	orderedDays := sortedDayKeys(days)
	events := make([]interface{}, 0, len(orderedDays)*9)
	var dailyCount, kCount int
	for _, dayKey := range orderedDays {
		observedDate := days[dayKey]
		solarRow, hasSolar := solarByDay[dayKey]
		geomagRow, hasGeomag := geomagByDay[dayKey]

		daily := DailyIndex{
			DayKey:       dayKey,
			ObservedDate: observedDate,
			Source:       source,
			LoadedAt:     loadedAt,
		}
		if hasSolar {
			daily.SFI = solarRow.SFI
			daily.SunspotNumber = solarRow.SunspotNumber
		}
		if hasGeomag {
			daily.AIndex = geomagRow.AIndex
			daily.APIndex = geomagRow.APIndex
		}
		events = append(events, NewDailyIndexEvent(daily))
		dailyCount++

		if !hasGeomag {
			continue
		}
		for i, kp := range geomagRow.KP {
			bucketStart := observedDate.Add(time.Duration(i*3) * time.Hour)
			events = append(events, NewKIndex3HEvent(KIndex3H{
				BucketKey:   ThreeHourBucketKeyUTC(bucketStart),
				BucketStart: bucketStart,
				DayKey:      dayKey,
				KIndex:      int(math.Round(kp)),
				KPIndex:     kp,
				Source:      source,
				LoadedAt:    loadedAt,
			}))
			kCount++
		}
	}
	return events, dailyCount, kCount
}

func NewDailyIndexEvent(row DailyIndex) DailyIndexEvent {
	return DailyIndexEvent{
		Type: DailyIndexEventType,
		Data: DailyIndexPayload{
			DayKey:        row.DayKey,
			ObservedDate:  formatTime(row.ObservedDate),
			AIndex:        row.AIndex,
			APIndex:       row.APIndex,
			SFI:           row.SFI,
			SunspotNumber: row.SunspotNumber,
			Source:        defaultSource(row.Source),
			LoadedAt:      formatTime(row.LoadedAt),
		},
	}
}

func NewKIndex3HEvent(row KIndex3H) KIndex3HEvent {
	return KIndex3HEvent{
		Type: KIndex3HEventType,
		Data: KIndex3HPayload{
			BucketKey:   row.BucketKey,
			BucketStart: formatTime(row.BucketStart),
			DayKey:      row.DayKey,
			KIndex:      row.KIndex,
			KPIndex:     row.KPIndex,
			Source:      defaultSource(row.Source),
			LoadedAt:    formatTime(row.LoadedAt),
		},
	}
}

func DecodeEventJSON(data []byte) (map[string]interface{}, error) {
	var event map[string]interface{}
	err := json.Unmarshal(data, &event)
	return event, err
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

func ParseDay(value string) (time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(value), time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	return day, nil
}

func inRange(t, from, to time.Time) bool {
	t = truncateDay(t)
	if !from.IsZero() && t.Before(truncateDay(from)) {
		return false
	}
	if !to.IsZero() && t.After(truncateDay(to)) {
		return false
	}
	return true
}

func sortedDayKeys(days map[int]time.Time) []int {
	keys := make([]int, 0, len(days))
	for key := range days {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func looksLikeDateFields(fields []string) bool {
	if len(fields) < 3 {
		return false
	}
	if len(fields[0]) != 4 || len(fields[1]) != 2 || len(fields[2]) != 2 {
		return false
	}
	_, err := parseDateFields(fields[:3])
	return err == nil
}

func parseDateFields(fields []string) (time.Time, error) {
	year, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, err
	}
	month, err := strconv.Atoi(fields[1])
	if err != nil {
		return time.Time{}, err
	}
	day, err := strconv.Atoi(fields[2])
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC), nil
}

func truncateDay(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func formatDay(t time.Time) string {
	return truncateDay(t).Format("2006-01-02")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func defaultSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "swpc"
	}
	return source
}
