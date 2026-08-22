package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(0)

	target := flag.String("target", "http://127.0.0.1:8088/ingest/json", "qstream-loader JSON ingest endpoint")
	batchSize := flag.Int("batch-size", 1000, "events per loader POST")
	limit := flag.Int("limit", 0, "maximum records to load; 0 means no limit")
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

	reader, archiveDate, err := rbn.OpenArchiveFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	client := rbn.LoaderClient{Target: *target}
	ctx := context.Background()
	batch := make([]rbn.SpotEvent, 0, *batchSize)
	var emitted, accepted, failed int
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		batchCount := len(batch)
		reqCtx, cancel := context.WithTimeout(ctx, *timeout)
		defer cancel()
		resp, err := client.PostEvents(reqCtx, batch)
		if err != nil {
			return err
		}
		if resp.Accepted+resp.Failed != batchCount {
			return fmt.Errorf("loader accounted for %d events, posted %d", resp.Accepted+resp.Failed, batchCount)
		}
		accepted += resp.Accepted
		failed += resp.Failed
		batch = batch[:0]
		return nil
	}

	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		if *limit > 0 && emitted >= *limit {
			return errLimitReached
		}
		batch = append(batch, rbn.NewSpotEvent(spot))
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

	fmt.Fprintf(os.Stderr, "rows=%d emitted=%d accepted=%d failed=%d rejected=%d skipped_footer=%d\n",
		stats.Rows, emitted, accepted, failed, stats.RejectedRows, stats.SkippedFooter)
	if failed > 0 {
		os.Exit(1)
	}
}

var errLimitReached = fmt.Errorf("limit reached")
