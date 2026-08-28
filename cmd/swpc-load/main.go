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
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/swpc"
)

const (
	defaultSolarSource  = "https://services.swpc.noaa.gov/text/daily-solar-indices.txt"
	defaultGeomagSource = "https://services.swpc.noaa.gov/text/daily-geomagnetic-indices.txt"
)

func main() {
	log.SetFlags(0)

	solarSource := flag.String("solar-source", defaultSolarSource, "SWPC daily solar indices URL, file path, or file:// URL")
	geomagSource := flag.String("geomag-source", defaultGeomagSource, "SWPC daily geomagnetic indices URL, file path, or file:// URL")
	fromValue := flag.String("from", "", "inclusive UTC start date, YYYY-MM-DD; empty means first parsed date")
	toValue := flag.String("to", "", "inclusive UTC end date, YYYY-MM-DD; empty means last parsed date")
	sourceLabel := flag.String("source-label", "swpc", "source label stored in emitted rows")
	target := flag.String("target", "", "optional qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 100, "events per loader POST when -target is set")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout when -target is set")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: swpc-load [flags]\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
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

	solarReader, err := openSource(*solarSource)
	if err != nil {
		log.Fatalf("open solar source: %v", err)
	}
	defer solarReader.Close()
	solarRows, err := swpc.ParseDailySolarIndices(solarReader)
	if err != nil {
		log.Fatalf("parse solar source: %v", err)
	}

	geomagReader, err := openSource(*geomagSource)
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

	accepted, failed, err := postEvents(context.Background(), *target, events, *batchSize, *timeout)
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

func openSource(source string) (io.ReadCloser, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if source == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequest(http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "radiosport-data-lab/0.1")
		resp, err := http.DefaultClient.Do(req)
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
