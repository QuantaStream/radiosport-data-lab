package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/QuantaStream/radiosport-data-lab/internal/qrz"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

var errLimitReached = errors.New("limit reached")

func main() {
	log.SetFlags(0)

	limit := flag.Int("limit", 0, "maximum records to emit; 0 means no limit")
	spotType := flag.String("spot-type", rbn.DefaultSpotEventType, "event type used for spot records")
	qrzParents := flag.Bool("qrz-parents", true, "emit pending qrz_callsign parent events before spots")
	activityParents := flag.Bool("activity-parents", true, "emit activity_5m_bucket parent events before spots")
	denseSpotIDs := flag.Bool("dense-spot-ids", false, "assign day-local dense spot ids for storage-friendly archive backfills")
	dxCallFilter := flag.String("dx-call", "", "optional DX callsign filter, for example TI8X")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: rbn-archive-to-jsonl [flags] <RBN daily .zip or .csv>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	normalizedDXCall := strings.TrimSpace(*dxCallFilter)
	if normalizedDXCall != "" {
		var ok bool
		normalizedDXCall, ok = rbn.NormalizeCallsign(normalizedDXCall)
		if !ok {
			log.Fatalf("invalid -dx-call %q", *dxCallFilter)
		}
	}

	path := flag.Arg(0)
	encoder := json.NewEncoder(os.Stdout)
	if *activityParents {
		parentEvents, err := collectActivityParentEvents(path, *limit, normalizedDXCall)
		if err != nil {
			log.Fatal(err)
		}
		for _, event := range parentEvents {
			if err := encoder.Encode(event); err != nil {
				log.Fatal(err)
			}
		}
	}

	reader, archiveDate, err := rbn.OpenArchiveFile(path)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	emitted := 0
	seenQRZ := map[string]struct{}{}
	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		if *limit > 0 && emitted >= *limit {
			return errLimitReached
		}
		if normalizedDXCall != "" && spot.DXCall != normalizedDXCall {
			return nil
		}
		if *qrzParents {
			if _, ok := seenQRZ[spot.DXCall]; !ok {
				seenQRZ[spot.DXCall] = struct{}{}
				if err := encoder.Encode(qrz.NewPendingProfileEvent(spot.DXCall)); err != nil {
					return err
				}
			}
		}
		if *denseSpotIDs {
			spot.SpotID = rbn.DenseArchiveSpotID(spot.SpottedAt, emitted)
		}
		if err := encoder.Encode(rbn.NewSpotEventWithType(spot, *spotType)); err != nil {
			return err
		}
		emitted++
		return nil
	})
	if err != nil && !errors.Is(err, errLimitReached) {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "rows=%d emitted=%d rejected=%d skipped_footer=%d\n", stats.Rows, emitted, stats.RejectedRows, stats.SkippedFooter)
}

func collectActivityParentEvents(path string, limit int, dxCallFilter string) ([]rbn.Activity5MBucketEvent, error) {
	reader, archiveDate, err := rbn.OpenArchiveFile(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	buckets := map[uint64]rbn.Activity5MBucket{}
	var emitted int
	_, err = rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		if limit > 0 && emitted >= limit {
			return errLimitReached
		}
		if dxCallFilter != "" && spot.DXCall != dxCallFilter {
			return nil
		}
		bucket := rbn.Activity5MBucketFromSpot(spot)
		buckets[bucket.Activity5MID] = bucket
		emitted++
		return nil
	})
	if err != nil && !errors.Is(err, errLimitReached) {
		return nil, err
	}

	ordered := rbn.SortedActivity5MBuckets(buckets)
	events := make([]rbn.Activity5MBucketEvent, 0, len(ordered))
	for _, bucket := range ordered {
		events = append(events, rbn.NewActivity5MBucketEvent(bucket))
	}
	return events, nil
}
