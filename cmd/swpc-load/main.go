package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/swpc"
)

const (
	defaultSolarSource        = "https://services.swpc.noaa.gov/text/daily-solar-indices.txt"
	defaultGeomagSource       = "https://services.swpc.noaa.gov/text/daily-geomagnetic-indices.txt"
	defaultHistoricalBaseURLs = "https://ftp.swpc.noaa.gov/pub/indices/old_indices,https://solar.physics.montana.edu/takeda/NOAA_reports/archive/{year}"
)

func main() {
	log.SetFlags(0)

	solarSource := flag.String("solar-source", defaultSolarSource, "SWPC daily solar indices URL, file path, or file:// URL")
	geomagSource := flag.String("geomag-source", defaultGeomagSource, "SWPC daily geomagnetic indices URL, file path, or file:// URL")
	year := flag.Int("year", 0, "historical SWPC year to load using YYYY_DSD.txt and YYYY_DGD.txt; explicit source flags override this")
	cacheDir := flag.String("cache-dir", "data/swpc", "cache directory for -year historical SWPC files")
	historicalBaseURL := flag.String("historical-base-url", defaultHistoricalBaseURLs, "comma-separated base URLs or URL templates for -year historical SWPC files; {year} is expanded")
	refreshCache := flag.Bool("refresh-cache", false, "redownload -year source files even when cached files exist")
	fromValue := flag.String("from", "", "inclusive UTC start date, YYYY-MM-DD; empty means first parsed date")
	toValue := flag.String("to", "", "inclusive UTC end date, YYYY-MM-DD; empty means last parsed date")
	sourceLabel := flag.String("source-label", "swpc", "source label stored in emitted rows")
	target := flag.String("target", "", "optional qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 100, "events per loader POST when -target is set")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP request timeout for source fetches and loader POSTs")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: swpc-load [flags]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	if *year < 0 {
		log.Fatal("-year must be zero or a positive year")
	}

	from, err := parseOptionalDay(*fromValue)
	if err != nil {
		log.Fatalf("parse -from: %v", err)
	}
	to, err := parseOptionalDay(*toValue)
	if err != nil {
		log.Fatalf("parse -to: %v", err)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		log.Fatal("-from must be before or equal to -to")
	}

	ctx := context.Background()
	solarSourceValue := *solarSource
	geomagSourceValue := *geomagSource
	if *year != 0 {
		if !flagWasSet("solar-source") {
			solarSourceValue, err = resolveYearlySource(ctx, "solar", *year, *cacheDir, *historicalBaseURL, *refreshCache, *timeout)
			if err != nil {
				log.Fatalf("resolve yearly solar source: %v", err)
			}
			log.Printf("yearly solar source=%s", solarSourceValue)
		}
		if !flagWasSet("geomag-source") {
			geomagSourceValue, err = resolveYearlySource(ctx, "geomag", *year, *cacheDir, *historicalBaseURL, *refreshCache, *timeout)
			if err != nil {
				log.Fatalf("resolve yearly geomag source: %v", err)
			}
			log.Printf("yearly geomag source=%s", geomagSourceValue)
		}
	}

	solarReader, err := openSource(ctx, solarSourceValue, *timeout)
	if err != nil {
		log.Fatalf("open solar source: %v", err)
	}
	defer solarReader.Close()
	solarRows, err := swpc.ParseDailySolarIndices(solarReader)
	if err != nil {
		log.Fatalf("parse solar source: %v", err)
	}

	geomagReader, err := openSource(ctx, geomagSourceValue, *timeout)
	if err != nil {
		log.Fatalf("open geomag source: %v", err)
	}
	defer geomagReader.Close()
	geomagRows, err := swpc.ParseDailyGeomagneticIndices(geomagReader)
	if err != nil {
		log.Fatalf("parse geomag source: %v", err)
	}

	events, dailyCount, kCount := swpc.BuildEvents(solarRows, geomagRows, from, to, *sourceLabel, time.Now().UTC())
	if len(events) == 0 {
		log.Fatalf("no SWPC rows matched requested date range; solar_rows=%d geomag_rows=%d", len(solarRows), len(geomagRows))
	}

	if strings.TrimSpace(*target) == "" {
		if err := writeJSONL(os.Stdout, events); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(os.Stderr, "solar_rows=%d geomag_rows=%d daily_events=%d k_events=%d emitted=%d\n",
			len(solarRows), len(geomagRows), dailyCount, kCount, len(events))
		return
	}

	accepted, failed, err := postEvents(ctx, *target, events, *batchSize, *timeout)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "solar_rows=%d geomag_rows=%d daily_events=%d k_events=%d accepted=%d failed=%d\n",
		len(solarRows), len(geomagRows), dailyCount, kCount, accepted, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func parseOptionalDay(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return swpc.ParseDay(value)
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

func flagWasSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func resolveYearlySource(ctx context.Context, product string, year int, cacheDir, baseURL string, refresh bool, timeout time.Duration) (string, error) {
	filename, err := yearlyFilename(product, year)
	if err != nil {
		return "", err
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("cache directory is required for -year")
	}
	path := filepath.Join(cacheDir, filename)
	if !refresh && fileHasContent(path) {
		return path, nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	urls := historicalSourceURLs(baseURL, filename, year)
	if len(urls) == 0 {
		return "", fmt.Errorf("historical base URL is required")
	}
	var attempts []string
	for _, sourceURL := range urls {
		if err := downloadSource(ctx, sourceURL, path, timeout); err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", sourceURL, err))
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf("download %s: all historical sources failed: %s", filename, strings.Join(attempts, "; "))
}

func yearlyFilename(product string, year int) (string, error) {
	if year <= 0 || year > 9999 {
		return "", fmt.Errorf("invalid historical SWPC year %d", year)
	}
	switch product {
	case "solar":
		return fmt.Sprintf("%04d_DSD.txt", year), nil
	case "geomag":
		return fmt.Sprintf("%04d_DGD.txt", year), nil
	default:
		return "", fmt.Errorf("unsupported SWPC historical product %q", product)
	}
}

func fileHasContent(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir() && stat.Size() > 0
}

func historicalSourceURLs(baseURLs, filename string, year int) []string {
	yearText := fmt.Sprintf("%04d", year)
	parts := strings.Split(baseURLs, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.ReplaceAll(part, "{year}", yearText)
		urls = append(urls, strings.TrimRight(part, "/")+"/"+filename)
	}
	return urls
}

func downloadSource(ctx context.Context, sourceURL, dest string, timeout time.Duration) error {
	if strings.TrimSpace(sourceURL) == "" {
		return fmt.Errorf("source URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "radiosport-data-lab/0.1")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s", sourceURL, resp.Status)
	}
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
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
	client := rbn.LoaderClient{
		Target: target,
		Client: &http.Client{Timeout: timeout},
	}
	var accepted, failed int
	for len(events) > 0 {
		end := batchSize
		if end > len(events) {
			end = len(events)
		}
		resp, err := client.PostEvents(ctx, events[:end])
		if err != nil {
			return accepted, failed, err
		}
		accepted += resp.Accepted
		failed += resp.Failed
		events = events[end:]
	}
	return accepted, failed, nil
}
