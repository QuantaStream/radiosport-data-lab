package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/qrz"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(0)

	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	workers := flag.Int("workers", 1, "concurrent loader POST workers; use with -qrz-parents=false for flat loads")
	dayWorkers := flag.Int("day-workers", 1, "concurrent archive day files to load")
	limit := flag.Int("limit", 0, "maximum records to load per archive file; 0 means no limit")
	spotType := flag.String("spot-type", rbn.DefaultSpotEventType, "event type used for spot records")
	qrzParents := flag.Bool("qrz-parents", true, "emit pending qrz_callsign parent events before spots")
	denseSpotIDs := flag.Bool("dense-spot-ids", false, "assign day-local dense spot ids for storage-friendly archive backfills")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: rbn-archive-load [flags] <RBN daily .zip or .csv> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	if *workers <= 0 {
		log.Fatal("-workers must be greater than zero")
	}
	if *dayWorkers <= 0 {
		log.Fatal("-day-workers must be greater than zero")
	}
	if (*workers > 1 || *dayWorkers > 1) && *qrzParents {
		log.Fatal("-workers or -day-workers greater than 1 requires -qrz-parents=false; relationship loads must preserve parent-before-spot order")
	}

	sort.Strings(paths)
	config := loadConfig{
		target:       *target,
		batchSize:    *batchSize,
		postWorkers:  *workers,
		limit:        *limit,
		spotType:     *spotType,
		qrzParents:   *qrzParents,
		denseSpotIDs: *denseSpotIDs,
		timeout:      *timeout,
	}

	startedAt := time.Now()
	results := loadArchives(context.Background(), config, paths, *dayWorkers)
	var rows, emitted, accepted, failed, rejectedRows, skippedFooter int
	var firstErr error
	for _, result := range results {
		rows += result.rows
		emitted += result.emitted
		accepted += result.accepted
		failed += result.failed
		rejectedRows += result.rejectedRows
		skippedFooter += result.skippedFooter
		if len(paths) > 1 {
			fmt.Fprintf(os.Stderr, "file=%s rows=%d emitted=%d accepted=%d failed=%d rejected=%d skipped_footer=%d elapsed=%s\n",
				result.path, result.rows, result.emitted, result.accepted, result.failed, result.rejectedRows,
				result.skippedFooter, result.elapsed.Round(time.Millisecond))
		}
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}
	}
	fmt.Fprintf(os.Stderr, "files=%d rows=%d emitted=%d accepted=%d failed=%d rejected=%d skipped_footer=%d elapsed=%s\n",
		len(paths), rows, emitted, accepted, failed, rejectedRows, skippedFooter, time.Since(startedAt).Round(time.Millisecond))
	if firstErr != nil {
		log.Fatal(firstErr)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

var errLimitReached = fmt.Errorf("limit reached")

func loadArchives(ctx context.Context, config loadConfig, paths []string, dayWorkers int) []archiveLoadResult {
	if dayWorkers > len(paths) {
		dayWorkers = len(paths)
	}
	if dayWorkers <= 1 {
		results := make([]archiveLoadResult, 0, len(paths))
		for _, path := range paths {
			results = append(results, loadArchive(ctx, config, path))
		}
		return results
	}

	loadCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan string)
	results := make(chan archiveLoadResult, len(paths))
	var wg sync.WaitGroup
	for i := 0; i < dayWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				result := loadArchive(loadCtx, config, path)
				if result.err != nil {
					cancel()
				}
				results <- result
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, path := range paths {
			select {
			case <-loadCtx.Done():
				return
			case jobs <- path:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	collected := make([]archiveLoadResult, 0, len(paths))
	for result := range results {
		collected = append(collected, result)
	}
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].path < collected[j].path
	})
	return collected
}

func loadArchive(ctx context.Context, config loadConfig, path string) archiveLoadResult {
	startedAt := time.Now()
	result := archiveLoadResult{path: path}
	reader, archiveDate, err := rbn.OpenArchiveFile(path)
	if err != nil {
		result.err = err
		return result
	}
	defer reader.Close()

	client := rbn.LoaderClient{Target: config.target}
	poster := newBatchPoster(ctx, client, config.timeout, config.postWorkers)
	defer poster.Close()
	batch := make([]interface{}, 0, config.batchSize)
	seenQRZ := map[string]struct{}{}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		events := make([]interface{}, len(batch))
		copy(events, batch)
		batch = batch[:0]
		return poster.Post(events)
	}

	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if config.limit > 0 && result.emitted >= config.limit {
			return errLimitReached
		}
		if config.qrzParents {
			if _, ok := seenQRZ[spot.DXCall]; !ok {
				seenQRZ[spot.DXCall] = struct{}{}
				batch = append(batch, qrz.NewPendingProfileEvent(spot.DXCall))
				if len(batch) >= config.batchSize {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
		if config.denseSpotIDs {
			spot.SpotID = rbn.DenseArchiveSpotID(spot.SpottedAt, result.emitted)
		}
		batch = append(batch, rbn.NewSpotEventWithType(spot, config.spotType))
		result.emitted++
		if len(batch) >= config.batchSize {
			return flush()
		}
		return nil
	})
	if err != nil && err != errLimitReached {
		result.err = err
		result.elapsed = time.Since(startedAt)
		return result
	}
	if err := flush(); err != nil {
		result.err = err
		result.elapsed = time.Since(startedAt)
		return result
	}
	accepted, failed, err := poster.Close()
	if err != nil {
		result.err = err
	}
	result.rows = stats.Rows
	result.accepted = accepted
	result.failed = failed
	result.rejectedRows = stats.RejectedRows
	result.skippedFooter = stats.SkippedFooter
	result.elapsed = time.Since(startedAt)
	if failed > 0 {
		result.err = fmt.Errorf("%s had %d loader failures", path, failed)
	}
	return result
}

type batchPoster struct {
	ctx       context.Context
	cancel    context.CancelFunc
	client    rbn.LoaderClient
	timeout   time.Duration
	workers   int
	accepted  int
	failed    int
	firstErr  error
	batches   chan []interface{}
	results   chan postResult
	done      chan struct{}
	collected chan struct{}
	doneOnce  sync.Once
}

type postResult struct {
	accepted int
	failed   int
	err      error
}

type loadConfig struct {
	target       string
	batchSize    int
	postWorkers  int
	limit        int
	spotType     string
	qrzParents   bool
	denseSpotIDs bool
	timeout      time.Duration
}

type archiveLoadResult struct {
	path          string
	rows          int
	emitted       int
	accepted      int
	failed        int
	rejectedRows  int
	skippedFooter int
	elapsed       time.Duration
	err           error
}

func newBatchPoster(ctx context.Context, client rbn.LoaderClient, timeout time.Duration, workers int) *batchPoster {
	posterCtx, cancel := context.WithCancel(ctx)
	p := &batchPoster{
		ctx:     posterCtx,
		cancel:  cancel,
		client:  client,
		timeout: timeout,
		workers: workers,
	}
	if workers <= 1 {
		return p
	}
	p.batches = make(chan []interface{}, workers*2)
	p.results = make(chan postResult, workers)
	p.done = make(chan struct{})
	p.collected = make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for events := range p.batches {
				result := p.post(events)
				if result.err != nil {
					p.cancel()
				}
				p.results <- result
			}
		}()
	}
	go func() {
		wg.Wait()
		close(p.results)
		close(p.done)
	}()
	go func() {
		for result := range p.results {
			if result.err != nil && p.firstErr == nil {
				p.firstErr = result.err
			}
			p.accepted += result.accepted
			p.failed += result.failed
		}
		close(p.collected)
	}()
	return p
}

func (p *batchPoster) Post(events []interface{}) error {
	if len(events) == 0 {
		return nil
	}
	if p.workers <= 1 {
		result := p.post(events)
		p.accepted += result.accepted
		p.failed += result.failed
		return result.err
	}
	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case p.batches <- events:
		return nil
	}
}

func (p *batchPoster) Close() (int, int, error) {
	if p == nil {
		return 0, 0, nil
	}
	p.doneOnce.Do(func() {
		if p.workers > 1 {
			close(p.batches)
			<-p.done
			<-p.collected
		}
		p.cancel()
	})
	return p.accepted, p.failed, p.firstErr
}

func (p *batchPoster) post(events []interface{}) postResult {
	reqCtx, cancel := context.WithTimeout(p.ctx, p.timeout)
	defer cancel()
	resp, err := p.client.PostEvents(reqCtx, events)
	if err != nil {
		return postResult{err: err}
	}
	if resp.Accepted+resp.Failed != len(events) {
		return postResult{err: fmt.Errorf("loader accounted for %d events, posted %d", resp.Accepted+resp.Failed, len(events))}
	}
	return postResult{accepted: resp.Accepted, failed: resp.Failed}
}
