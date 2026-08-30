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
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/contestmatch"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbncache"
)

func main() {
	log.SetFlags(0)

	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	ctyPath := flag.String("cty-dat", "", "optional CTY/DXCC data file; empty searches RBN_CTY_DAT and data/cty/cty.dat")
	contestID := flag.String("contest-id", "", "contest id override; empty derives from Cabrillo CONTEST and QSO year")
	scopeRegion := flag.String("scope-region", "tier1", "scope label stored on parsed contest rows")
	sourceFile := flag.String("source-file", "", "source label override for Cabrillo parsing")
	rbnCache := flag.String("rbn-cache", "", "RBN parsed cache root; when set, RBN arguments are YYYY-MM-DD or YYYYMMDD cache days")
	window := flag.Duration("window", 5*time.Minute, "spot/QSO match window on each side of the QSO time")
	frequencyToleranceKHz := flag.Float64("frequency-tolerance-khz", 0, "maximum absolute frequency delta; 0 disables frequency filtering")
	maxMatchesPerQSO := flag.Int("max-matches-per-qso", 0, "maximum closest spot matches per QSO; 0 keeps all matches")
	denseSpotIDs := flag.Bool("dense-spot-ids", true, "assign the same day-local dense spot ids used by archive loads")
	denseMatchIDs := flag.Bool("dense-match-ids", true, "assign contiguous match_id values for compact QS storage")
	spotterProfileDSN := flag.String("spotter-profile-mysql-dsn", "", "optional QS MySQL-wire DSN used to read spotter_profiles for calibrated signal metrics")
	spotterProfileTable := flag.String("spotter-profile-table", "spotter_profiles", "QS table containing current spotter calibration profiles")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: contest-spot-match-load [flags] <Cabrillo log path or URL> <RBN daily .zip or .csv> [...]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "       contest-spot-match-load -rbn-cache <cache-dir> [flags] <Cabrillo log path or URL> <YYYY-MM-DD> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(2)
	}
	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	if *window <= 0 {
		log.Fatal("-window must be greater than zero")
	}
	if *frequencyToleranceKHz < 0 {
		log.Fatal("-frequency-tolerance-khz cannot be negative")
	}
	if *maxMatchesPerQSO < 0 {
		log.Fatal("-max-matches-per-qso cannot be negative")
	}

	db, err := loadCallsignDB(*ctyPath)
	if err != nil {
		log.Printf("cty enrichment disabled: %v", err)
	}

	ctx := context.Background()
	contestLog, qsos, err := loadContestLog(ctx, flag.Arg(0), *timeout, *sourceFile, *contestID, *scopeRegion, db)
	if err != nil {
		log.Fatal(err)
	}
	spots, sourceStats, err := loadSpotSources(ctx, flag.Args()[1:], contestLog.StationCall, *denseSpotIDs, *rbnCache)
	if err != nil {
		log.Fatal(err)
	}
	spotterProfiles, err := loadSpotterProfiles(ctx, *spotterProfileDSN, *spotterProfileTable)
	if err != nil {
		log.Printf("spotter profile calibration disabled: %v", err)
	}
	if len(spotterProfiles) > 0 {
		log.Printf("spotter profile calibration loaded profiles=%d table=%s", len(spotterProfiles), *spotterProfileTable)
	}
	matches := contestmatch.MatchQSOsToSpots(qsos, spots, contestmatch.Options{
		Window:                *window,
		FrequencyToleranceKHz: *frequencyToleranceKHz,
		MaxMatchesPerQSO:      *maxMatchesPerQSO,
		DenseMatchIDs:         *denseMatchIDs,
		LoadedAt:              time.Now().UTC(),
		SpotterProfiles:       spotterProfiles,
	})
	events := contestmatch.Events(matches)

	if strings.TrimSpace(*target) == "" {
		if err := writeJSONL(os.Stdout, events); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "log_id=%s station=%s qsos=%d spot_source=%s source_rows=%d spots=%d matches=%d emitted=%d rejected=%d skipped_footer=%d\n",
			contestLog.LogID, contestLog.StationCall, len(qsos), sourceStats.Source, sourceStats.Rows, len(spots), len(matches), len(events), sourceStats.RejectedRows, sourceStats.SkippedFooter)
		return
	}

	accepted, failed, err := postEvents(ctx, *target, events, *batchSize, *timeout)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "log_id=%s station=%s qsos=%d spot_source=%s source_rows=%d spots=%d matches=%d accepted=%d failed=%d rejected=%d skipped_footer=%d\n",
		contestLog.LogID, contestLog.StationCall, len(qsos), sourceStats.Source, sourceStats.Rows, len(spots), len(matches), accepted, failed, sourceStats.RejectedRows, sourceStats.SkippedFooter)
	if failed > 0 {
		os.Exit(1)
	}
}

type spotSourceStats struct {
	Source        string
	Rows          int
	RejectedRows  int
	SkippedFooter int
}

func loadSpotterProfiles(ctx context.Context, dsn string, table string) (map[string]contestmatch.SpotterProfile, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, nil
	}
	table = strings.TrimSpace(table)
	if err := validateIdentifier(table); err != nil {
		return nil, fmt.Errorf("-spotter-profile-table: %w", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, fmt.Sprintf("select spotter_call, spotter_weight, normalization_offset_db, profile_quality from %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	profiles := map[string]contestmatch.SpotterProfile{}
	for rows.Next() {
		var call sql.NullString
		var weight sql.NullFloat64
		var baseline sql.NullFloat64
		var quality sql.NullString
		if err := rows.Scan(&call, &weight, &baseline, &quality); err != nil {
			return nil, err
		}
		normalizedCall, ok := rbn.NormalizeCallsign(nullString(call))
		if !ok {
			continue
		}
		profileQuality := strings.TrimSpace(quality.String)
		if !quality.Valid || profileQuality == "" {
			profileQuality = "unknown"
		}
		profiles[normalizedCall] = contestmatch.SpotterProfile{
			SpotterWeight: nullFloat(weight, 1),
			BaselineDB:    nullFloat(baseline, 0),
			Quality:       profileQuality,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profiles, nil
}

func loadContestLog(ctx context.Context, source string, timeout time.Duration, sourceFile string, contestID string, scopeRegion string, db *callsign.Database) (cabrillo.Log, []cabrillo.QSO, error) {
	reader, err := openSource(ctx, source, timeout)
	if err != nil {
		return cabrillo.Log{}, nil, fmt.Errorf("open Cabrillo log: %w", err)
	}
	defer reader.Close()
	label := strings.TrimSpace(sourceFile)
	if label == "" {
		label = cabrillo.SourceLabel(source)
	}
	contestLog, qsos, stats, err := cabrillo.Parse(reader, cabrillo.ParseOptions{
		ContestID:   contestID,
		ScopeRegion: scopeRegion,
		SourceFile:  label,
		LoadedAt:    time.Now().UTC(),
		CallsignDB:  db,
	})
	if err != nil {
		return cabrillo.Log{}, nil, err
	}
	if stats.RejectedQSOs > 0 {
		log.Printf("cabrillo rejected_qsos=%d parsed_qsos=%d", stats.RejectedQSOs, stats.ParsedQSOs)
	}
	return contestLog, qsos, nil
}

func loadSpotSources(ctx context.Context, args []string, stationCall string, denseSpotIDs bool, cacheDir string) ([]rbn.Spot, spotSourceStats, error) {
	if strings.TrimSpace(cacheDir) != "" {
		return loadCachedSpots(ctx, cacheDir, args, stationCall)
	}
	return loadArchiveSpots(ctx, args, stationCall, denseSpotIDs)
}

func loadArchiveSpots(ctx context.Context, paths []string, stationCall string, denseSpotIDs bool) ([]rbn.Spot, spotSourceStats, error) {
	var allSpots []rbn.Spot
	aggregate := spotSourceStats{Source: "archive"}
	for _, path := range paths {
		reader, archiveDate, err := rbn.OpenArchiveFile(path)
		if err != nil {
			return nil, aggregate, err
		}
		var emitted int
		stats, readErr := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if spot.DXCall != stationCall {
				return nil
			}
			if denseSpotIDs {
				spot.SpotID = rbn.DenseArchiveSpotID(spot.SpottedAt, emitted)
			}
			allSpots = append(allSpots, spot)
			emitted++
			return nil
		})
		closeErr := reader.Close()
		if readErr != nil {
			return nil, aggregate, readErr
		}
		if closeErr != nil {
			return nil, aggregate, closeErr
		}
		aggregate.Rows += stats.Rows
		aggregate.RejectedRows += stats.RejectedRows
		aggregate.SkippedFooter += stats.SkippedFooter
		fmt.Fprintf(os.Stderr, "archive=%s rows=%d matched_spots=%d rejected=%d skipped_footer=%d\n",
			path, stats.Rows, emitted, stats.RejectedRows, stats.SkippedFooter)
	}
	return allSpots, aggregate, nil
}

func loadCachedSpots(ctx context.Context, cacheDir string, dayArgs []string, stationCall string) ([]rbn.Spot, spotSourceStats, error) {
	if len(dayArgs) == 0 {
		return nil, spotSourceStats{}, fmt.Errorf("at least one cache day is required when -rbn-cache is set")
	}
	call, ok := rbn.NormalizeCallsign(stationCall)
	if !ok {
		return nil, spotSourceStats{}, fmt.Errorf("invalid station callsign %q", stationCall)
	}

	var allSpots []rbn.Spot
	aggregate := spotSourceStats{Source: "rbn-cache"}
	for _, dayArg := range dayArgs {
		day, err := rbncache.ParseCacheDay(dayArg)
		if err != nil {
			return nil, aggregate, err
		}
		spots, stats, err := rbncache.ReadCallSpots(ctx, cacheDir, day, call)
		if err != nil {
			return nil, aggregate, err
		}
		allSpots = append(allSpots, spots...)
		aggregate.Rows += stats.Records
		fmt.Fprintf(os.Stderr, "cache_day=%s call=%s path=%s spots=%d\n",
			day.Format("2006-01-02"), call, stats.Path, stats.Records)
	}
	return allSpots, aggregate, nil
}

func loadCallsignDB(path string) (*callsign.Database, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		return callsign.LoadFile(path)
	}
	db, _, err := callsign.LoadDefault()
	return db, err
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

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullFloat(value sql.NullFloat64, fallback float64) float64 {
	if !value.Valid {
		return fallback
	}
	return value.Float64
}

func openSource(ctx context.Context, source string, timeout time.Duration) (io.ReadCloser, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if source == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "radiosport-data-lab/0.1")
		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: %s", source, resp.Status)
		}
		return resp.Body, nil
	}
	if strings.HasPrefix(source, "file://") {
		parsed, err := url.Parse(source)
		if err != nil {
			return nil, err
		}
		source = parsed.Path
	}
	return os.Open(source)
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

func postEvents(ctx context.Context, target string, events []interface{}, batchSize int, timeout time.Duration) (int, int, error) {
	client := rbn.LoaderClient{Target: target}
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
