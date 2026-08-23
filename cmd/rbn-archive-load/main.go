package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
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
	limit := flag.Int("limit", 0, "maximum records to load; 0 means no limit")
	spotType := flag.String("spot-type", rbn.DefaultSpotEventType, "event type used for spot records")
	qrzParents := flag.Bool("qrz-parents", true, "emit pending qrz_callsign parent events before spots")
	denseSpotIDs := flag.Bool("dense-spot-ids", false, "assign day-local dense spot ids for storage-friendly archive backfills")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: rbn-archive-load [flags] <RBN daily .zip or .csv>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	if *workers <= 0 {
		log.Fatal("-workers must be greater than zero")
	}
	if *workers > 1 && *qrzParents {
		log.Fatal("-workers greater than 1 requires -qrz-parents=false; relationship loads must preserve parent-before-spot order")
	}

	reader, archiveDate, err := rbn.OpenArchiveFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	client := rbn.LoaderClient{Target: *target}
	ctx := context.Background()
	poster := newBatchPoster(ctx, client, *timeout, *workers)
	defer poster.Close()
	batch := make([]interface{}, 0, *batchSize)
	seenQRZ := map[string]struct{}{}
	var emitted int
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
		if *limit > 0 && emitted >= *limit {
			return errLimitReached
		}
		if *qrzParents {
			if _, ok := seenQRZ[spot.DXCall]; !ok {
				seenQRZ[spot.DXCall] = struct{}{}
				batch = append(batch, qrz.NewPendingProfileEvent(spot.DXCall))
				if len(batch) >= *batchSize {
					if err := flush(); err != nil {
						return err
					}
				}
			}
		}
		if *denseSpotIDs {
			spot.SpotID = rbn.DenseArchiveSpotID(spot.SpottedAt, emitted)
		}
		batch = append(batch, rbn.NewSpotEventWithType(spot, *spotType))
		emitted++
		if len(batch) >= *batchSize {
			return flush()
		}
		return nil
	})
	if err != nil && err != errLimitReached {
		log.Fatal(err)
	}
	if err := flush(); err != nil {
		log.Fatal(err)
	}
	accepted, failed, err := poster.Close()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stderr, "rows=%d emitted=%d accepted=%d failed=%d rejected=%d skipped_footer=%d\n",
		stats.Rows, emitted, accepted, failed, stats.RejectedRows, stats.SkippedFooter)
	if failed > 0 {
		os.Exit(1)
	}
}

var errLimitReached = fmt.Errorf("limit reached")

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
