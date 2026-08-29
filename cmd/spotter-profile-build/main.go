package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/geo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/spotterprofile"
)

func main() {
	log.SetFlags(0)

	mysqlDSN := flag.String("mysql-dsn", "qstream@tcp(127.0.0.1:4000)/quanta?parseTime=true", "QS MySQL-wire DSN used to read source spots")
	sourceTable := flag.String("source-table", spotterprofile.DefaultSourceTable, "source QS spot table")
	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	parentFlushWait := flag.Duration("parent-flush-wait", 2*time.Second, "wait after posting generated spotter node parents before posting profile rows")
	ctyPath := flag.String("cty-dat", "", "optional CTY/DXCC data file for spotter country centroid enrichment; empty searches RBN_CTY_DAT and data/cty/cty.dat")
	from := flag.String("from", "", "inclusive UTC window start, such as 2025-11-29 or 2025-11-29T00:00:00Z")
	to := flag.String("to", "", "inclusive UTC query window end, such as 2025-12-01 or 2025-12-01T00:00:00Z")
	profileKind := flag.String("profile-kind", spotterprofile.DefaultProfileKind, "profile kind label")
	minGoodSpots := flag.Int("min-good-spots", 25, "minimum spot count for profile_quality=good")
	minGoodHours := flag.Int("min-good-hours", 2, "minimum active hour count for profile_quality=good")
	flag.Parse()

	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	windowStart, err := parseOptionalTime(*from)
	if err != nil {
		log.Fatalf("parse -from: %v", err)
	}
	windowEnd, err := parseOptionalTime(*to)
	if err != nil {
		log.Fatalf("parse -to: %v", err)
	}
	callsignDB, err := loadCallsignDB(*ctyPath)
	if err != nil {
		log.Printf("cty spotter geo enrichment disabled: %v", err)
	}

	ctx := context.Background()
	observations, observedStart, observedEnd, err := readObservations(ctx, *mysqlDSN, *sourceTable, windowStart, windowEnd, callsignDB)
	if err != nil {
		log.Fatal(err)
	}
	if windowStart.IsZero() {
		windowStart = observedStart
	}
	if windowEnd.IsZero() {
		windowEnd = observedEnd
	}
	summaries := spotterprofile.BuildSummaries(observations, spotterprofile.Options{
		ProfileKind:  *profileKind,
		SourceTable:  *sourceTable,
		WindowStart:  windowStart,
		WindowEnd:    windowEnd,
		ComputedAt:   time.Now().UTC(),
		MinGoodSpots: *minGoodSpots,
		MinGoodHours: *minGoodHours,
		ProfileBand:  spotterprofile.AllBands,
		ProfileMode:  spotterprofile.AllModes,
	})
	for _, summary := range summaries {
		if err := spotterprofile.ValidateSummary(summary); err != nil {
			log.Fatal(err)
		}
	}
	nodeEvents := spotterprofile.NodeEvents(summaries)
	snapshotEvents := spotterprofile.SnapshotEvents(summaries)
	profileEvents := spotterprofile.ProfileEvents(summaries)
	events := make([]interface{}, 0, len(nodeEvents)+len(snapshotEvents)+len(profileEvents))
	events = append(events, nodeEvents...)
	events = append(events, snapshotEvents...)
	events = append(events, profileEvents...)

	if strings.TrimSpace(*target) == "" {
		if err := writeJSONL(os.Stdout, events); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "source_table=%s observations=%d spotters=%d emitted=%d window_start=%s window_end=%s\n",
			*sourceTable, len(observations), len(summaries), len(events), formatLogTime(windowStart), formatLogTime(windowEnd))
		return
	}

	accepted, failed, err := postEvents(ctx, *target, nodeEvents, *batchSize, *timeout)
	if err != nil {
		log.Fatal(err)
	}
	if len(nodeEvents) > 0 && *parentFlushWait > 0 {
		time.Sleep(*parentFlushWait)
	}
	childAccepted, childFailed, err := postEvents(ctx, *target, append(snapshotEvents, profileEvents...), *batchSize, *timeout)
	if err != nil {
		log.Fatal(err)
	}
	accepted += childAccepted
	failed += childFailed
	fmt.Fprintf(os.Stderr, "source_table=%s observations=%d spotters=%d accepted=%d failed=%d window_start=%s window_end=%s\n",
		*sourceTable, len(observations), len(summaries), accepted, failed, formatLogTime(windowStart), formatLogTime(windowEnd))
	if failed > 0 {
		os.Exit(1)
	}
}

func readObservations(ctx context.Context, dsn string, sourceTable string, windowStart time.Time, windowEnd time.Time, callsignDB *callsign.Database) ([]spotterprofile.Observation, time.Time, time.Time, error) {
	sourceTable = strings.TrimSpace(sourceTable)
	if err := validateIdentifier(sourceTable); err != nil {
		return nil, time.Time{}, time.Time{}, fmt.Errorf("-source-table: %w", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, time.Time{}, time.Time{}, err
	}

	query := buildSourceQuery(sourceTable, windowStart, windowEnd)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	defer rows.Close()

	var observations []spotterprofile.Observation
	var observedStart time.Time
	var observedEnd time.Time
	enrichmentCache := map[string]spotterEnrichment{}
	for rows.Next() {
		var spotterCall sql.NullString
		var spotterPrefix sql.NullString
		var spotterContinent sql.NullString
		var dxCall sql.NullString
		var dxPrefix sql.NullString
		var signalDB sql.NullInt64
		var spottedAt any
		if err := rows.Scan(&spotterCall, &spotterPrefix, &spotterContinent, &dxCall, &dxPrefix, &signalDB, &spottedAt); err != nil {
			return nil, time.Time{}, time.Time{}, err
		}
		ts, err := parseDBTime(spottedAt)
		if err != nil {
			return nil, time.Time{}, time.Time{}, err
		}
		if observedStart.IsZero() || ts.Before(observedStart) {
			observedStart = ts
		}
		if observedEnd.IsZero() || ts.After(observedEnd) {
			observedEnd = ts
		}
		spotterCallValue := nullString(spotterCall)
		enrichedSpotter, ok := enrichmentCache[spotterCallValue]
		if !ok {
			enrichedSpotter = enrichSpotter(callsignDB, spotterCallValue)
			enrichmentCache[spotterCallValue] = enrichedSpotter
		}
		spotterPrefixValue := nullString(spotterPrefix)
		if spotterPrefixValue == "" || strings.EqualFold(spotterPrefixValue, rbn.UnknownValue) {
			spotterPrefixValue = enrichedSpotter.prefix
		}
		spotterContinentValue := nullString(spotterContinent)
		if spotterContinentValue == "" || strings.EqualFold(spotterContinentValue, rbn.UnknownValue) {
			spotterContinentValue = enrichedSpotter.continent
		}
		observations = append(observations, spotterprofile.Observation{
			SpotterCall:          spotterCallValue,
			SpotterPrefix:        spotterPrefixValue,
			SpotterContinent:     spotterContinentValue,
			SpotterCountry:       enrichedSpotter.country,
			SpotterCQZone:        enrichedSpotter.cqZone,
			SpotterITUZone:       enrichedSpotter.ituZone,
			SpotterLatitude:      enrichedSpotter.latitude,
			SpotterLongitude:     enrichedSpotter.longitude,
			SpotterGeoSource:     enrichedSpotter.geoSource,
			SpotterGeoConfidence: enrichedSpotter.geoConfidence,
			DXCall:               nullString(dxCall),
			DXPrefix:             nullString(dxPrefix),
			SignalDB:             int(signalDB.Int64),
			SpottedAt:            ts,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, time.Time{}, err
	}
	return observations, observedStart, observedEnd, nil
}

type spotterEnrichment struct {
	prefix        string
	continent     string
	country       string
	cqZone        int
	ituZone       int
	latitude      float64
	longitude     float64
	geoSource     string
	geoConfidence string
}

func enrichSpotter(db *callsign.Database, call string) spotterEnrichment {
	unknownLocation := geo.Unknown()
	enrichment := spotterEnrichment{
		prefix:        rbn.UnknownValue,
		continent:     rbn.UnknownValue,
		country:       rbn.UnknownValue,
		geoSource:     unknownLocation.Source,
		geoConfidence: unknownLocation.Confidence,
	}
	if db == nil {
		return enrichment
	}
	station, err := db.Parse(call)
	if err != nil || !station.Valid {
		return enrichment
	}
	enrichment.prefix = defaultString(station.Prefix, rbn.UnknownValue)
	enrichment.continent = defaultString(station.Continent, rbn.UnknownValue)
	enrichment.country = defaultString(station.Country, rbn.UnknownValue)
	enrichment.cqZone = station.CQZone
	enrichment.ituZone = station.ITUZone
	location := geo.FromCTYCountry(station.Latitude, station.Longitude)
	enrichment.latitude = location.Latitude
	enrichment.longitude = location.Longitude
	enrichment.geoSource = location.Source
	enrichment.geoConfidence = location.Confidence
	return enrichment
}

func loadCallsignDB(path string) (*callsign.Database, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		return callsign.LoadFile(path)
	}
	db, _, err := callsign.LoadDefault()
	return db, err
}

func buildSourceQuery(sourceTable string, windowStart time.Time, windowEnd time.Time) string {
	query := fmt.Sprintf(`select spotter_call, spotter_prefix, spotter_continent, dx_call, dx_prefix, signal_db, spotted_at from %s`, sourceTable)
	if !windowStart.IsZero() || !windowEnd.IsZero() {
		if windowStart.IsZero() {
			windowStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if windowEnd.IsZero() {
			windowEnd = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
		}
		query += fmt.Sprintf(" where spotted_at between todate('%s') and todate('%s')", formatSQLTime(windowStart), formatSQLTime(windowEnd))
	}
	query += " order by spotter_call"
	return query
}

func validateIdentifier(value string) error {
	if value == "" {
		return fmt.Errorf("value is required")
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return fmt.Errorf("unsupported identifier %q", value)
		}
	}
	return nil
}

func postEvents(ctx context.Context, target string, events []interface{}, batchSize int, timeout time.Duration) (int, int, error) {
	client := rbn.LoaderClient{Target: target, Client: &http.Client{Timeout: timeout}}
	var accepted, failed int
	for start := 0; start < len(events); start += batchSize {
		end := start + batchSize
		if end > len(events) {
			end = len(events)
		}
		reqCtx, cancel := context.WithTimeout(ctx, timeout)
		resp, err := client.PostEvents(reqCtx, events[start:end])
		cancel()
		if err != nil {
			return accepted, failed, err
		}
		accepted += resp.Accepted
		failed += resp.Failed
	}
	return accepted, failed, nil
}

func writeJSONL(w io.Writer, events []interface{}) error {
	encoder := json.NewEncoder(w)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	return nil
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", value)
}

func parseDBTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v.UTC(), nil
	case []byte:
		return parseOptionalTime(string(v))
	case string:
		return parseOptionalTime(v)
	case nil:
		return time.Time{}, fmt.Errorf("timestamp is null")
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func formatSQLTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func formatLogTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
