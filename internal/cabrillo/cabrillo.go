package cabrillo

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/geo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const UnknownValue = rbn.UnknownValue

type ParseOptions struct {
	ContestID   string
	ScopeRegion string
	SourceFile  string
	LoadedAt    time.Time
	CallsignDB  *callsign.Database
}

type ParseStats struct {
	Lines        int
	HeaderLines  int
	QSOLines     int
	ParsedQSOs   int
	RejectedQSOs int
}

type Log struct {
	LogID                string
	ContestID            string
	StationCall          string
	StationPrefix        string
	StationContinent     string
	StationCountry       string
	CQZone               int
	ITUZone              int
	StationLatitude      float64
	StationLongitude     float64
	StationGeoSource     string
	StationGeoConfidence string
	CategoryOperator     string
	CategoryAssisted     string
	CategoryBand         string
	CategoryPower        string
	CategoryMode         string
	CategoryTransmitter  string
	ClaimedScore         int64
	QSOCount             int
	ScopeRegion          string
	SourceFile           string
	LoadedAt             time.Time
}

type QSO struct {
	QSOID            uint64
	LogID            string
	ContestID        string
	QSOAt            time.Time
	QSODayKey        int
	QSO3HBucketKey   int
	QSO5MBucketKey   int
	Activity5MID     uint64
	Activity5MKey    string
	StationCall      string
	StationPrefix    string
	StationContinent string
	WorkedCall       string
	WorkedPrefix     string
	WorkedContinent  string
	FrequencyKHz     float64
	Band             string
	Mode             string
	SentExchange     string
	ReceivedExchange string
	SourceFile       string
}

type callInfo struct {
	Prefix        string
	Continent     string
	Country       string
	CQZone        int
	ITUZone       int
	Latitude      float64
	Longitude     float64
	GeoSource     string
	GeoConfidence string
}

func Parse(r io.Reader, options ParseOptions) (Log, []QSO, ParseStats, error) {
	headers := map[string]string{}
	var stats ParseStats
	var rawQSOs []rawQSO

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		stats.Lines++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(line), "QSO:") {
			stats.QSOLines++
			qso, err := parseQSOLine(line, stats.Lines)
			if err != nil {
				stats.RejectedQSOs++
				continue
			}
			rawQSOs = append(rawQSOs, qso)
			stats.ParsedQSOs++
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(name))
		if key == "" || key == "END-OF-LOG" {
			continue
		}
		if _, exists := headers[key]; !exists {
			headers[key] = strings.TrimSpace(value)
		}
		stats.HeaderLines++
	}
	if err := scanner.Err(); err != nil {
		return Log{}, nil, stats, err
	}
	if len(rawQSOs) == 0 {
		return Log{}, nil, stats, fmt.Errorf("Cabrillo log contains no parseable QSO rows")
	}

	sourceFile := strings.TrimSpace(options.SourceFile)
	if sourceFile == "" {
		sourceFile = "UNKNOWN"
	}
	loadedAt := options.LoadedAt.UTC()
	if loadedAt.IsZero() {
		loadedAt = time.Now().UTC()
	}

	stationCall := normalizeHeaderCall(headers["CALLSIGN"])
	if stationCall == "" {
		stationCall = rawQSOs[0].stationCall
	}
	if stationCall == "" {
		return Log{}, nil, stats, fmt.Errorf("Cabrillo log has no station callsign")
	}
	station := enrichCall(options.CallsignDB, stationCall)
	contestID := contestID(headers, rawQSOs[0].qsoAt, options.ContestID)
	logID := stableLogID(contestID, stationCall)

	log := Log{
		LogID:                logID,
		ContestID:            contestID,
		StationCall:          stationCall,
		StationPrefix:        station.Prefix,
		StationContinent:     station.Continent,
		StationCountry:       station.Country,
		CQZone:               station.CQZone,
		ITUZone:              station.ITUZone,
		StationLatitude:      station.Latitude,
		StationLongitude:     station.Longitude,
		StationGeoSource:     station.GeoSource,
		StationGeoConfidence: station.GeoConfidence,
		CategoryOperator:     normalizeCategory(headers["CATEGORY-OPERATOR"]),
		CategoryAssisted:     normalizeCategory(headers["CATEGORY-ASSISTED"]),
		CategoryBand:         normalizeCategory(headers["CATEGORY-BAND"]),
		CategoryPower:        normalizeCategory(headers["CATEGORY-POWER"]),
		CategoryMode:         normalizeCategory(headers["CATEGORY-MODE"]),
		CategoryTransmitter:  normalizeCategory(headers["CATEGORY-TRANSMITTER"]),
		ClaimedScore:         parseInt64Default(headers["CLAIMED-SCORE"]),
		QSOCount:             len(rawQSOs),
		ScopeRegion:          normalizeCategory(defaultString(options.ScopeRegion, "tier1")),
		SourceFile:           sourceFile,
		LoadedAt:             loadedAt,
	}

	qsos := make([]QSO, 0, len(rawQSOs))
	for _, raw := range rawQSOs {
		worked := enrichCall(options.CallsignDB, raw.workedCall)
		qso := QSO{
			LogID:            log.LogID,
			ContestID:        log.ContestID,
			QSOAt:            raw.qsoAt,
			QSODayKey:        rbn.DayKeyUTC(raw.qsoAt),
			QSO3HBucketKey:   rbn.ThreeHourBucketKeyUTC(raw.qsoAt),
			QSO5MBucketKey:   rbn.FiveMinuteBucketKeyUTC(raw.qsoAt),
			StationCall:      raw.stationCall,
			StationPrefix:    station.Prefix,
			StationContinent: station.Continent,
			WorkedCall:       raw.workedCall,
			WorkedPrefix:     worked.Prefix,
			WorkedContinent:  worked.Continent,
			FrequencyKHz:     raw.frequencyKHz,
			Band:             raw.band,
			Mode:             raw.mode,
			SentExchange:     raw.sentExchange,
			ReceivedExchange: raw.receivedExchange,
			SourceFile:       sourceFile,
		}
		qso.Activity5MKey = rbn.Activity5MKey(qso.StationCall, qso.Band, qso.Mode, qso.QSOAt)
		qso.Activity5MID = rbn.Activity5MID(qso.Activity5MKey)
		qso.QSOID = stableQSOID(qso, raw.lineNumber)
		qsos = append(qsos, qso)
	}
	return log, qsos, stats, nil
}

type rawQSO struct {
	lineNumber       int
	frequencyKHz     float64
	band             string
	mode             string
	qsoAt            time.Time
	stationCall      string
	workedCall       string
	sentExchange     string
	receivedExchange string
}

func parseQSOLine(line string, lineNumber int) (rawQSO, error) {
	fields := strings.Fields(line)
	if len(fields) < 11 || !strings.EqualFold(fields[0], "QSO:") {
		return rawQSO{}, fmt.Errorf("line %d is not a supported Cabrillo QSO row", lineNumber)
	}
	frequencyKHz, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return rawQSO{}, fmt.Errorf("line %d frequency: %w", lineNumber, err)
	}
	band, ok := rbn.BandForFrequencyKHz(frequencyKHz)
	if !ok {
		band = UnknownValue
	}
	qsoAt, err := time.ParseInLocation("2006-01-02 1504", fields[3]+" "+fields[4], time.UTC)
	if err != nil {
		return rawQSO{}, fmt.Errorf("line %d timestamp: %w", lineNumber, err)
	}
	stationCall, ok := rbn.NormalizeCallsign(fields[5])
	if !ok {
		return rawQSO{}, fmt.Errorf("line %d station callsign %q", lineNumber, fields[5])
	}
	workedCall, ok := rbn.NormalizeCallsign(fields[8])
	if !ok {
		return rawQSO{}, fmt.Errorf("line %d worked callsign %q", lineNumber, fields[8])
	}
	return rawQSO{
		lineNumber:       lineNumber,
		frequencyKHz:     frequencyKHz,
		band:             band,
		mode:             normalizeCategory(fields[2]),
		qsoAt:            qsoAt,
		stationCall:      stationCall,
		workedCall:       workedCall,
		sentExchange:     normalizeCategory(fields[7]),
		receivedExchange: normalizeCategory(fields[10]),
	}, nil
}

func enrichCall(db *callsign.Database, call string) callInfo {
	unknownLocation := geo.Unknown()
	info := callInfo{
		Prefix:        UnknownValue,
		Continent:     UnknownValue,
		Country:       UnknownValue,
		GeoSource:     unknownLocation.Source,
		GeoConfidence: unknownLocation.Confidence,
	}
	if db == nil {
		return info
	}
	station, err := db.Parse(call)
	if err != nil || !station.Valid {
		return info
	}
	info.Prefix = defaultString(station.Prefix, UnknownValue)
	info.Continent = defaultString(station.Continent, UnknownValue)
	info.Country = defaultString(station.Country, UnknownValue)
	info.CQZone = station.CQZone
	info.ITUZone = station.ITUZone
	location := geo.FromCTYCountry(station.Latitude, station.Longitude)
	info.Latitude = location.Latitude
	info.Longitude = location.Longitude
	info.GeoSource = location.Source
	info.GeoConfidence = location.Confidence
	return info
}

func contestID(headers map[string]string, firstQSO time.Time, override string) string {
	if strings.TrimSpace(override) != "" {
		return normalizeIdentifier(override)
	}
	contest := normalizeIdentifier(headers["CONTEST"])
	if contest == "" || contest == "unknown" {
		contest = "contest"
	}
	if !firstQSO.IsZero() {
		return fmt.Sprintf("%s-%04d", contest, firstQSO.UTC().Year())
	}
	return contest
}

func stableLogID(contestID string, stationCall string) string {
	return normalizeIdentifier(contestID) + ":" + strings.ToUpper(strings.TrimSpace(stationCall))
}

func stableQSOID(qso QSO, lineNumber int) uint64 {
	h := fnv.New64a()
	writePart := func(v string) {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	writePart(qso.LogID)
	writePart(strconv.Itoa(lineNumber))
	writePart(qso.QSOAt.UTC().Format(time.RFC3339))
	writePart(qso.StationCall)
	writePart(qso.WorkedCall)
	writePart(strconv.FormatFloat(qso.FrequencyKHz, 'f', 1, 64))
	writePart(qso.Mode)
	writePart(qso.SentExchange)
	writePart(qso.ReceivedExchange)
	id := h.Sum64() & ((uint64(1) << 63) - 1)
	if id == 0 {
		return 1
	}
	return id
}

func normalizeHeaderCall(input string) string {
	call, ok := rbn.NormalizeCallsign(input)
	if !ok {
		return ""
	}
	return call
}

func normalizeCategory(input string) string {
	value := strings.ToUpper(strings.TrimSpace(input))
	if value == "" {
		return "UNSPECIFIED"
	}
	return value
}

func normalizeIdentifier(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	if value == "" {
		return "unknown"
	}
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}

func parseInt64Default(input string) int64 {
	value := strings.TrimSpace(input)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func SourceLabel(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return UnknownValue
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return source
	}
	base := filepath.Base(source)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return source
	}
	return base
}
