package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/QuantaStream/radiosport-data-lab/internal/qrz"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

var errLimitReached = errors.New("limit reached")

func main() {
	log.SetFlags(0)

	limit := flag.Int("limit", 0, "maximum records to emit; 0 means no limit")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: rbn-archive-to-jsonl [flags] <RBN daily .zip or .csv>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	reader, archiveDate, err := rbn.OpenArchiveFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	encoder := json.NewEncoder(os.Stdout)
	emitted := 0
	seenQRZ := map[string]struct{}{}
	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		if *limit > 0 && emitted >= *limit {
			return errLimitReached
		}
		if _, ok := seenQRZ[spot.DXCall]; !ok {
			seenQRZ[spot.DXCall] = struct{}{}
			if err := encoder.Encode(qrz.NewPendingProfileEvent(spot.DXCall)); err != nil {
				return err
			}
		}
		if err := encoder.Encode(rbn.NewSpotEvent(spot)); err != nil {
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
