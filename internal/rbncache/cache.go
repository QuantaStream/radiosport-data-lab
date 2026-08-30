package rbncache

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const ManifestVersion = 1

type BuildOptions struct {
	CacheDir     string
	SpotType     string
	DXCalls      []string
	DenseSpotIDs bool
	Now          func() time.Time
}

type BuildResult struct {
	Manifest Manifest
	Path     string
}

type Manifest struct {
	Version       int         `json:"version"`
	BuiltAt       string      `json:"built_at"`
	Source        string      `json:"source"`
	ArchiveDate   string      `json:"archive_date"`
	SpotType      string      `json:"spot_type"`
	DenseSpotIDs  bool        `json:"dense_spot_ids"`
	DXCallFilters []string    `json:"dx_call_filters"`
	Rows          int         `json:"rows"`
	RejectedRows  int         `json:"rejected_rows"`
	SkippedFooter int         `json:"skipped_footer"`
	Emitted       int         `json:"emitted"`
	DXCalls       []CallEntry `json:"dx_calls"`
}

type CallEntry struct {
	DXCall         string `json:"dx_call"`
	File           string `json:"file"`
	Spots          int    `json:"spots"`
	FirstSpottedAt string `json:"first_spotted_at"`
	LastSpottedAt  string `json:"last_spotted_at"`
}

type ReadStats struct {
	Path    string
	DXCall  string
	Records int
}

func BuildArchive(ctx context.Context, path string, options BuildOptions) (BuildResult, error) {
	cacheDir := strings.TrimSpace(options.CacheDir)
	if cacheDir == "" {
		return BuildResult{}, fmt.Errorf("cache directory is required")
	}
	spotType := strings.TrimSpace(options.SpotType)
	if spotType == "" {
		spotType = rbn.FlatSpotEventType
	}
	filters, err := NormalizeDXCalls(options.DXCalls)
	if err != nil {
		return BuildResult{}, err
	}
	if len(filters) == 0 {
		return BuildResult{}, fmt.Errorf("at least one DX callsign filter is required for focused RBN cache builds")
	}

	reader, archiveDate, err := rbn.OpenArchiveFile(path)
	if err != nil {
		return BuildResult{}, err
	}
	defer reader.Close()
	if archiveDate.IsZero() {
		return BuildResult{}, fmt.Errorf("archive date could not be derived from %s", path)
	}

	dayDir := DayDir(cacheDir, archiveDate)
	byCallDir := filepath.Join(dayDir, "by_dx_call")
	if err := os.MkdirAll(byCallDir, 0o755); err != nil {
		return BuildResult{}, err
	}
	for _, call := range filters {
		_ = os.Remove(filepath.Join(byCallDir, callFileName(call)))
	}

	writer := focusedWriter{
		dir:      byCallDir,
		spotType: spotType,
		calls:    map[string]*callWriter{},
	}
	defer writer.Close()
	for _, call := range filters {
		if err := writer.Ensure(call); err != nil {
			return BuildResult{}, err
		}
	}

	filterSet := map[string]struct{}{}
	for _, call := range filters {
		filterSet[call] = struct{}{}
	}

	var emitted int
	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, ok := filterSet[spot.DXCall]; !ok {
			return nil
		}
		if options.DenseSpotIDs {
			spot.SpotID = rbn.DenseArchiveSpotID(spot.SpottedAt, emitted)
		}
		emitted++
		return writer.Write(spot)
	})
	if err != nil {
		return BuildResult{}, err
	}
	if err := writer.Close(); err != nil {
		return BuildResult{}, err
	}

	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	manifest := Manifest{
		Version:       ManifestVersion,
		BuiltAt:       now().UTC().Format(time.RFC3339),
		Source:        path,
		ArchiveDate:   archiveDate.UTC().Format("2006-01-02"),
		SpotType:      spotType,
		DenseSpotIDs:  options.DenseSpotIDs,
		DXCallFilters: filters,
		Rows:          stats.Rows,
		RejectedRows:  stats.RejectedRows,
		SkippedFooter: stats.SkippedFooter,
		Emitted:       emitted,
		DXCalls:       writer.Entries(),
	}
	manifestPath := filepath.Join(dayDir, "manifest.json")
	if err := writeManifest(manifestPath, manifest); err != nil {
		return BuildResult{}, err
	}
	return BuildResult{Manifest: manifest, Path: manifestPath}, nil
}

func ReadCallSpots(ctx context.Context, cacheDir string, day time.Time, dxCall string) ([]rbn.Spot, ReadStats, error) {
	path, call, err := CallPath(cacheDir, day, dxCall)
	if err != nil {
		return nil, ReadStats{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ReadStats{Path: path, DXCall: call}, nil
		}
		return nil, ReadStats{}, err
	}
	defer file.Close()

	var spots []rbn.Spot
	decoder := json.NewDecoder(file)
	for {
		select {
		case <-ctx.Done():
			return nil, ReadStats{}, ctx.Err()
		default:
		}
		var event rbn.SpotEvent
		if err := decoder.Decode(&event); err == io.EOF {
			break
		} else if err != nil {
			return nil, ReadStats{}, err
		}
		spot, err := spotFromPayload(event.Data)
		if err != nil {
			return nil, ReadStats{}, err
		}
		if spot.DXCall != call {
			return nil, ReadStats{}, fmt.Errorf("cache %s contains dx_call %q, want %q", path, spot.DXCall, call)
		}
		spots = append(spots, spot)
	}
	return spots, ReadStats{Path: path, DXCall: call, Records: len(spots)}, nil
}

func ParseCacheDay(input string) (time.Time, error) {
	value := strings.TrimSpace(input)
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if day, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return day, nil
		}
	}
	return time.Time{}, fmt.Errorf("cache day %q must be YYYY-MM-DD or YYYYMMDD", input)
}

func NormalizeDXCalls(calls []string) ([]string, error) {
	seen := map[string]struct{}{}
	var normalized []string
	for _, value := range calls {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			call, ok := rbn.NormalizeCallsign(part)
			if !ok {
				return nil, fmt.Errorf("invalid DX callsign %q", part)
			}
			if _, exists := seen[call]; exists {
				continue
			}
			seen[call] = struct{}{}
			normalized = append(normalized, call)
		}
	}
	sort.Strings(normalized)
	return normalized, nil
}

func DayDir(cacheDir string, day time.Time) string {
	utc := day.UTC()
	return filepath.Join(cacheDir, fmt.Sprintf("%04d", utc.Year()), fmt.Sprintf("%02d", utc.Month()), fmt.Sprintf("%02d", utc.Day()))
}

func CallPath(cacheDir string, day time.Time, dxCall string) (string, string, error) {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", "", fmt.Errorf("cache directory is required")
	}
	call, ok := rbn.NormalizeCallsign(dxCall)
	if !ok {
		return "", "", fmt.Errorf("invalid DX callsign %q", dxCall)
	}
	if day.IsZero() {
		return "", "", fmt.Errorf("cache day is required")
	}
	return filepath.Join(DayDir(cacheDir, day), "by_dx_call", callFileName(call)), call, nil
}

func writeManifest(path string, manifest Manifest) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}

type focusedWriter struct {
	dir      string
	spotType string
	calls    map[string]*callWriter
	closed   bool
}

func (w *focusedWriter) Write(spot rbn.Spot) error {
	if w.closed {
		return fmt.Errorf("cache writer is closed")
	}
	cw, err := w.writerFor(spot.DXCall)
	if err != nil {
		return err
	}
	return cw.Write(rbn.NewSpotEventWithType(spot, w.spotType))
}

func (w *focusedWriter) Ensure(call string) error {
	_, err := w.writerFor(call)
	return err
}

func (w *focusedWriter) writerFor(call string) (*callWriter, error) {
	if w.closed {
		return nil, fmt.Errorf("cache writer is closed")
	}
	cw, ok := w.calls[call]
	if !ok {
		path := filepath.Join(w.dir, callFileName(call))
		file, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		cw = &callWriter{
			dxCall: call,
			file:   file,
			writer: bufio.NewWriter(file),
			path:   path,
		}
		cw.encoder = json.NewEncoder(cw.writer)
		w.calls[call] = cw
	}
	return cw, nil
}

func (w *focusedWriter) Close() error {
	if w == nil || w.closed {
		return nil
	}
	w.closed = true
	var firstErr error
	for _, callWriter := range w.calls {
		if err := callWriter.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *focusedWriter) Entries() []CallEntry {
	entries := make([]CallEntry, 0, len(w.calls))
	for _, callWriter := range w.calls {
		entries = append(entries, callWriter.Entry())
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].DXCall < entries[j].DXCall
	})
	return entries
}

type callWriter struct {
	dxCall  string
	file    *os.File
	writer  *bufio.Writer
	encoder *json.Encoder
	path    string
	spots   int
	first   time.Time
	last    time.Time
}

func (w *callWriter) Write(event rbn.SpotEvent) error {
	if err := w.encoder.Encode(event); err != nil {
		return err
	}
	t := eventTime(event)
	if w.spots == 0 || t.Before(w.first) {
		w.first = t
	}
	if w.spots == 0 || t.After(w.last) {
		w.last = t
	}
	w.spots++
	return nil
}

func (w *callWriter) Close() error {
	err1 := w.writer.Flush()
	err2 := w.file.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (w *callWriter) Entry() CallEntry {
	return CallEntry{
		DXCall:         w.dxCall,
		File:           filepath.Base(w.path),
		Spots:          w.spots,
		FirstSpottedAt: formatTime(w.first),
		LastSpottedAt:  formatTime(w.last),
	}
}

func eventTime(event rbn.SpotEvent) time.Time {
	t, _ := time.Parse(time.RFC3339, event.Data.SpottedAt)
	return t
}

func spotFromPayload(payload rbn.SpotPayload) (rbn.Spot, error) {
	spottedAt, err := time.Parse(time.RFC3339, payload.SpottedAt)
	if err != nil {
		return rbn.Spot{}, fmt.Errorf("parse spotted_at %q: %w", payload.SpottedAt, err)
	}
	dxCall, ok := rbn.NormalizeCallsign(payload.DXCall)
	if !ok {
		return rbn.Spot{}, fmt.Errorf("invalid cached dx_call %q", payload.DXCall)
	}
	spotterCall, ok := rbn.NormalizeCallsign(payload.SpotterCall)
	if !ok {
		return rbn.Spot{}, fmt.Errorf("invalid cached spotter_call %q", payload.SpotterCall)
	}
	return rbn.Spot{
		SpotID:           payload.SpotID,
		SpottedAt:        spottedAt.UTC(),
		SpotDayKey:       payload.SpotDayKey,
		Spot3HBucketKey:  payload.Spot3HBucketKey,
		Spot5MBucketKey:  payload.Spot5MBucketKey,
		Activity5MID:     payload.Activity5MID,
		Activity5MKey:    payload.Activity5MKey,
		SpotterCall:      spotterCall,
		SpotterPrefix:    payload.SpotterPrefix,
		SpotterContinent: payload.SpotterContinent,
		DXCall:           dxCall,
		DXPrefix:         payload.DXPrefix,
		DXContinent:      payload.DXContinent,
		FrequencyKHz:     payload.FrequencyKHz,
		Band:             payload.Band,
		Mode:             payload.Mode,
		SignalDB:         payload.SignalDB,
		SpeedWPM:         payload.SpeedWPM,
		TransmitMode:     payload.TransmitMode,
		Source:           payload.Source,
	}, nil
}

func callFileName(call string) string {
	return url.PathEscape(call) + ".jsonl"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
