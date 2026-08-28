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

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(0)

	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint; empty writes JSONL to stdout")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	ctyPath := flag.String("cty-dat", "", "optional CTY/DXCC data file; empty searches RBN_CTY_DAT and data/cty/cty.dat")
	contestID := flag.String("contest-id", "", "contest id override; empty derives from Cabrillo CONTEST and QSO year")
	scopeRegion := flag.String("scope-region", "tier1", "scope label stored on contest_log rows")
	sourceFile := flag.String("source-file", "", "source label override stored in QS rows")
	activityParents := flag.Bool("activity-parents", true, "emit activity_5m_bucket parent events before contest_qso child rows")
	parentFlushWait := flag.Duration("parent-flush-wait", 2*time.Second, "wait after posting parent rows before posting contest_qso child rows")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: cabrillo-load [flags] <Cabrillo log path or URL> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}

	db, err := loadCallsignDB(*ctyPath)
	if err != nil {
		log.Printf("cty enrichment disabled: %v", err)
	}

	ctx := context.Background()
	var totalLogs, totalQSOs, totalAccepted, totalFailed int
	for _, source := range flag.Args() {
		reader, err := openSource(ctx, source, *timeout)
		if err != nil {
			log.Fatalf("open %s: %v", source, err)
		}
		label := strings.TrimSpace(*sourceFile)
		if label == "" {
			label = cabrillo.SourceLabel(source)
		}
		loadedAt := time.Now().UTC()
		contestLog, qsos, stats, err := cabrillo.Parse(reader, cabrillo.ParseOptions{
			ContestID:   *contestID,
			ScopeRegion: *scopeRegion,
			SourceFile:  label,
			LoadedAt:    loadedAt,
			CallsignDB:  db,
		})
		closeErr := reader.Close()
		if err != nil {
			log.Fatalf("parse %s: %v", source, err)
		}
		if closeErr != nil {
			log.Fatalf("close %s: %v", source, closeErr)
		}
		events := cabrillo.NewEventsWithActivityParents(contestLog, qsos, *activityParents)

		if strings.TrimSpace(*target) == "" {
			if err := writeJSONL(os.Stdout, events); err != nil {
				log.Fatal(err)
			}
			fmt.Fprintf(os.Stderr, "source=%s log_id=%s qsos=%d rejected=%d emitted=%d\n",
				source, contestLog.LogID, len(qsos), stats.RejectedQSOs, len(events))
			continue
		}

		parentEvents := []interface{}{cabrillo.NewLogEvent(contestLog)}
		if *activityParents {
			parentEvents = append(parentEvents, cabrillo.NewActivity5MBucketEvents(qsos)...)
		}
		parentAccepted, parentFailed, err := postEvents(ctx, *target, parentEvents, *batchSize, *timeout)
		if err != nil {
			log.Fatal(err)
		}
		if parentFailed > 0 {
			fmt.Fprintf(os.Stderr, "source=%s log_id=%s qsos=%d rejected=%d accepted=%d failed=%d\n",
				source, contestLog.LogID, len(qsos), stats.RejectedQSOs, parentAccepted, parentFailed)
			os.Exit(1)
		}
		if *parentFlushWait > 0 {
			time.Sleep(*parentFlushWait)
		}
		qsoEvents := make([]interface{}, 0, len(qsos))
		for _, qso := range qsos {
			qsoEvents = append(qsoEvents, cabrillo.NewQSOEvent(qso))
		}
		qsoAccepted, qsoFailed, err := postEvents(ctx, *target, qsoEvents, *batchSize, *timeout)
		if err != nil {
			log.Fatal(err)
		}
		accepted := parentAccepted + qsoAccepted
		failed := parentFailed + qsoFailed
		fmt.Fprintf(os.Stderr, "source=%s log_id=%s qsos=%d rejected=%d accepted=%d failed=%d\n",
			source, contestLog.LogID, len(qsos), stats.RejectedQSOs, accepted, failed)
		totalLogs++
		totalQSOs += len(qsos)
		totalAccepted += accepted
		totalFailed += failed
		if failed > 0 {
			os.Exit(1)
		}
	}
	if strings.TrimSpace(*target) != "" {
		fmt.Fprintf(os.Stderr, "logs=%d qsos=%d accepted=%d failed=%d\n", totalLogs, totalQSOs, totalAccepted, totalFailed)
	}
}

func loadCallsignDB(path string) (*callsign.Database, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		return callsign.LoadFile(path)
	}
	db, _, err := callsign.LoadDefault()
	return db, err
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
