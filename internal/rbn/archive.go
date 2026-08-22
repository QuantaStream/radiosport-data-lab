package rbn

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

var ArchiveHeader = []string{
	"callsign",
	"de_pfx",
	"de_cont",
	"freq",
	"band",
	"dx",
	"dx_pfx",
	"dx_cont",
	"mode",
	"db",
	"date",
	"speed",
	"tx_mode",
}

const archiveTimeLayout = "2006-01-02 15:04:05"

type ArchiveStats struct {
	Rows          int
	RejectedRows  int
	SkippedFooter int
}

func ReadArchiveCSV(r io.Reader, emit func(Spot) error) (ArchiveStats, error) {
	return ReadArchiveCSVWithDate(r, time.Time{}, emit)
}

func ReadArchiveCSVWithDate(r io.Reader, archiveDate time.Time, emit func(Spot) error) (ArchiveStats, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return ArchiveStats{}, fmt.Errorf("read archive header: %w", err)
	}
	if !sameHeader(header, ArchiveHeader) {
		return ArchiveStats{}, fmt.Errorf("unexpected archive header: got %v", header)
	}

	var stats ArchiveStats
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return stats, nil
		}
		if err != nil {
			stats.RejectedRows++
			continue
		}
		if isArchiveFooter(record) {
			stats.SkippedFooter++
			continue
		}
		spot, err := ParseArchiveRecordWithDate(record, archiveDate)
		if err != nil {
			stats.RejectedRows++
			continue
		}
		if err := emit(spot); err != nil {
			return stats, err
		}
		stats.Rows++
	}
}

func ParseArchiveRecord(record []string) (Spot, error) {
	return ParseArchiveRecordWithDate(record, time.Time{})
}

func ParseArchiveRecordWithDate(record []string, archiveDate time.Time) (Spot, error) {
	if len(record) != len(ArchiveHeader) {
		return Spot{}, fmt.Errorf("archive row has %d fields, want %d", len(record), len(ArchiveHeader))
	}

	spotterCall, ok := NormalizeCallsign(record[0])
	if !ok {
		return Spot{}, fmt.Errorf("invalid spotter callsign %q", record[0])
	}
	dxCall, ok := NormalizeCallsign(record[5])
	if !ok {
		return Spot{}, fmt.Errorf("invalid dx callsign %q", record[5])
	}

	freqKHz, err := strconv.ParseFloat(strings.TrimSpace(record[3]), 64)
	if err != nil {
		return Spot{}, fmt.Errorf("parse frequency %q: %w", record[3], err)
	}
	signalDB, err := strconv.Atoi(strings.TrimSpace(record[9]))
	if err != nil {
		return Spot{}, fmt.Errorf("parse db %q: %w", record[9], err)
	}
	speedWPM, err := strconv.Atoi(strings.TrimSpace(record[11]))
	if err != nil {
		return Spot{}, fmt.Errorf("parse speed %q: %w", record[11], err)
	}
	spottedAt, err := parseArchiveTimestamp(strings.TrimSpace(record[10]), archiveDate)
	if err != nil {
		return Spot{}, fmt.Errorf("parse date %q: %w", record[10], err)
	}

	spot := Spot{
		SpottedAt:        spottedAt,
		SpotterCall:      spotterCall,
		SpotterPrefix:    normalizeCode(record[1]),
		SpotterContinent: normalizeCode(record[2]),
		DXCall:           dxCall,
		DXPrefix:         normalizeCode(record[6]),
		DXContinent:      normalizeCode(record[7]),
		FrequencyKHz:     freqKHz,
		Band:             strings.TrimSpace(record[4]),
		Mode:             strings.TrimSpace(record[8]),
		SignalDB:         signalDB,
		SpeedWPM:         speedWPM,
		TransmitMode:     strings.TrimSpace(record[12]),
		Source:           SourceArchive,
	}
	spot.SpotID = StableSpotID(spot)
	return spot, nil
}

func parseArchiveTimestamp(value string, archiveDate time.Time) (time.Time, error) {
	for _, layout := range []string{
		archiveTimeLayout,
		"2006-01-02 15:04",
		time.RFC3339,
	} {
		if ts, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return ts, nil
		}
	}
	if archiveDate.IsZero() {
		return time.Time{}, fmt.Errorf("full timestamp required when archive date is unknown")
	}

	archiveDate = archiveDate.UTC()
	for _, layout := range []string{"15:04:05", "15:04"} {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return time.Date(archiveDate.Year(), archiveDate.Month(), archiveDate.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
		}
	}
	if len(value) == 5 && strings.HasSuffix(value, "Z") {
		value = value[:4]
	}
	if len(value) == 4 {
		hour, err := strconv.Atoi(value[:2])
		if err != nil {
			return time.Time{}, err
		}
		minute, err := strconv.Atoi(value[2:])
		if err != nil {
			return time.Time{}, err
		}
		return time.Date(archiveDate.Year(), archiveDate.Month(), archiveDate.Day(), hour, minute, 0, 0, time.UTC), nil
	}
	return time.Time{}, fmt.Errorf("unsupported archive timestamp")
}

func sameHeader(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.TrimSpace(got[i]) != want[i] {
			return false
		}
	}
	return true
}

func isArchiveFooter(record []string) bool {
	if len(record) != 1 {
		return false
	}
	value := strings.TrimSpace(record[0])
	return strings.HasPrefix(value, "(") && strings.HasSuffix(value, " rows)")
}

func normalizeCode(input string) string {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return UnknownValue
	}
	return value
}
