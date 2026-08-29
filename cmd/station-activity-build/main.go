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

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/stationactivity"
)

func main() {
	log.SetFlags(0)

	mysqlDSN := flag.String("mysql-dsn", "qstream@tcp(127.0.0.1:4000)/quanta?parseTime=true", "QS MySQL-wire DSN used to read source spots")
	sourceTable := flag.String("source-table", stationactivity.DefaultSourceTable, "source QS spot table")
	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	from := flag.String("from", "", "inclusive UTC window start, such as 2025-11-29 or 2025-11-29T00:00:00Z")
	to := flag.String("to", "", "inclusive UTC query window end, such as 2025-12-01 or 2025-12-01T00:00:00Z")
	dxCall := flag.String("dx-call", "", "optional DX callsign filter")
	emitAllContinent := flag.Bool("emit-all-continent", true, "emit an ALL spotter-continent rollup for each activity group")
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
	normalizedDXCall, err := normalizeOptionalCallsign(*dxCall)
	if err != nil {
		log.Fatalf("parse -dx-call: %v", err)
	}

	ctx := context.Background()
	summaries, observations, observedStart, observedEnd, err := buildSummaries(ctx, buildOptions{
		DSN:              *mysqlDSN,
		SourceTable:      *sourceTable,
		WindowStart:      windowStart,
		WindowEnd:        windowEnd,
		DXCall:           normalizedDXCall,
		EmitAllContinent: *emitAllContinent,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, summary := range summaries {
		if err := stationactivity.ValidateSummary(summary); err != nil {
			log.Fatal(err)
		}
	}
	events := stationactivity.Events(summaries)

	if strings.TrimSpace(*target) == "" {
		if err := writeJSONL(os.Stdout, events); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "source_table=%s observations=%d summaries=%d window_start=%s window_end=%s dx_call=%s\n",
			*sourceTable, observations, len(summaries), formatLogTime(observedStart), formatLogTime(observedEnd), normalizedDXCall)
		return
	}

	accepted, failed, err := postEvents(ctx, *target, events, *batchSize, *timeout)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "source_table=%s observations=%d summaries=%d accepted=%d failed=%d window_start=%s window_end=%s dx_call=%s\n",
		*sourceTable, observations, len(summaries), accepted, failed, formatLogTime(observedStart), formatLogTime(observedEnd), normalizedDXCall)
	if failed > 0 {
		os.Exit(1)
	}
}

type buildOptions struct {
	DSN              string
	SourceTable      string
	WindowStart      time.Time
	WindowEnd        time.Time
	DXCall           string
	EmitAllContinent bool
}

func buildSummaries(ctx context.Context, options buildOptions) ([]stationactivity.Summary, int, time.Time, time.Time, error) {
	sourceTable := strings.TrimSpace(options.SourceTable)
	if err := validateIdentifier(sourceTable); err != nil {
		return nil, 0, time.Time{}, time.Time{}, fmt.Errorf("-source-table: %w", err)
	}

	db, err := sql.Open("mysql", options.DSN)
	if err != nil {
		return nil, 0, time.Time{}, time.Time{}, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, 0, time.Time{}, time.Time{}, err
	}

	query := buildSourceQuery(sourceTable, options.WindowStart, options.WindowEnd, options.DXCall)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, 0, time.Time{}, time.Time{}, err
	}
	defer rows.Close()

	accumulator := stationactivity.NewAccumulator(stationactivity.Options{
		SourceTable:      sourceTable,
		ComputedAt:       time.Now().UTC(),
		EmitAllContinent: options.EmitAllContinent,
	})
	var observations int
	var observedStart time.Time
	var observedEnd time.Time
	for rows.Next() {
		observation, err := scanObservation(rows)
		if err != nil {
			return nil, 0, time.Time{}, time.Time{}, err
		}
		observations++
		if observedStart.IsZero() || observation.SpottedAt.Before(observedStart) {
			observedStart = observation.SpottedAt
		}
		if observedEnd.IsZero() || observation.SpottedAt.After(observedEnd) {
			observedEnd = observation.SpottedAt
		}
		accumulator.Add(observation)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, time.Time{}, time.Time{}, err
	}
	return accumulator.Summaries(), observations, observedStart, observedEnd, nil
}

func buildSourceQuery(sourceTable string, windowStart time.Time, windowEnd time.Time, dxCall string) string {
	query := fmt.Sprintf(`select
  dx_call,
  dx_prefix,
  dx_continent,
  activity_5m_id,
  activity_5m_key,
  spot_5m_bucket_key,
  spotted_at,
  band,
  mode,
  transmit_mode,
  spotter_call,
  spotter_prefix,
  spotter_continent,
  signal_db
from %s`, sourceTable)
	var predicates []string
	if !windowStart.IsZero() || !windowEnd.IsZero() {
		if windowStart.IsZero() {
			windowStart = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if windowEnd.IsZero() {
			windowEnd = time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
		}
		predicates = append(predicates, fmt.Sprintf("spotted_at between todate('%s') and todate('%s')", formatSQLTime(windowStart), formatSQLTime(windowEnd)))
	}
	if dxCall != "" {
		predicates = append(predicates, fmt.Sprintf("dx_call = '%s'", dxCall))
	}
	if len(predicates) > 0 {
		query += "\nwhere " + strings.Join(predicates, "\n  and ")
	}
	return query
}

func scanObservation(rows *sql.Rows) (stationactivity.Observation, error) {
	var dxCall sql.NullString
	var dxPrefix sql.NullString
	var dxContinent sql.NullString
	var activity5MID sql.NullInt64
	var activity5MKey sql.NullString
	var bucketKey sql.NullInt64
	var spottedAt any
	var band sql.NullString
	var mode sql.NullString
	var transmitMode sql.NullString
	var spotterCall sql.NullString
	var spotterPrefix sql.NullString
	var spotterContinent sql.NullString
	var signalDB sql.NullInt64
	if err := rows.Scan(
		&dxCall,
		&dxPrefix,
		&dxContinent,
		&activity5MID,
		&activity5MKey,
		&bucketKey,
		&spottedAt,
		&band,
		&mode,
		&transmitMode,
		&spotterCall,
		&spotterPrefix,
		&spotterContinent,
		&signalDB,
	); err != nil {
		return stationactivity.Observation{}, err
	}
	ts, err := parseDBTime(spottedAt)
	if err != nil {
		return stationactivity.Observation{}, err
	}
	return stationactivity.Observation{
		DXCall:           nullString(dxCall),
		DXPrefix:         nullString(dxPrefix),
		DXContinent:      nullString(dxContinent),
		Activity5MID:     stationactivity.ParseUint64Signed(activity5MID.Int64),
		Activity5MKey:    nullString(activity5MKey),
		BucketKey:        int(bucketKey.Int64),
		SpottedAt:        ts,
		Band:             nullString(band),
		Mode:             stationActivityMode(mode, transmitMode),
		SpotterCall:      nullString(spotterCall),
		SpotterPrefix:    nullString(spotterPrefix),
		SpotterContinent: nullString(spotterContinent),
		SignalDB:         int(signalDB.Int64),
	}, nil
}

func stationActivityMode(mode sql.NullString, transmitMode sql.NullString) string {
	if transmit := nullString(transmitMode); transmit != "" {
		return transmit
	}
	return nullString(mode)
}

func normalizeOptionalCallsign(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	call, ok := rbn.NormalizeCallsign(value)
	if !ok {
		return "", fmt.Errorf("invalid callsign %q", value)
	}
	return call, nil
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

func formatSQLTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

func formatLogTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
