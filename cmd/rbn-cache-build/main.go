package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbncache"
)

func main() {
	log.SetFlags(0)

	cacheDir := flag.String("cache-dir", "data/cache/rbn", "RBN parsed cache root")
	spotType := flag.String("spot-type", rbn.FlatSpotEventType, "cached spot event type")
	dxCalls := flag.String("dx-call", "", "comma-separated focused DX callsigns to cache, for example TI8X")
	denseSpotIDs := flag.Bool("dense-spot-ids", true, "assign day-local dense spot ids compatible with focused archive loads")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: rbn-cache-build [flags] <RBN daily .zip or .csv> [...]\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	calls, err := rbncache.NormalizeDXCalls([]string{*dxCalls})
	if err != nil {
		log.Fatal(err)
	}
	if len(calls) == 0 {
		log.Fatal("-dx-call is required for focused cache builds")
	}

	sort.Strings(paths)
	startedAt := time.Now()
	var rows, emitted, rejected, skipped int
	for _, path := range paths {
		result, err := rbncache.BuildArchive(context.Background(), path, rbncache.BuildOptions{
			CacheDir:     *cacheDir,
			SpotType:     *spotType,
			DXCalls:      calls,
			DenseSpotIDs: *denseSpotIDs,
		})
		if err != nil {
			log.Fatal(err)
		}
		rows += result.Manifest.Rows
		emitted += result.Manifest.Emitted
		rejected += result.Manifest.RejectedRows
		skipped += result.Manifest.SkippedFooter
		fmt.Fprintf(os.Stderr, "cache=%s source=%s archive_date=%s rows=%d emitted=%d dx_calls=%s rejected=%d skipped_footer=%d\n",
			result.Path,
			path,
			result.Manifest.ArchiveDate,
			result.Manifest.Rows,
			result.Manifest.Emitted,
			formatCallCounts(result.Manifest.DXCalls),
			result.Manifest.RejectedRows,
			result.Manifest.SkippedFooter,
		)
	}
	fmt.Fprintf(os.Stderr, "files=%d rows=%d emitted=%d rejected=%d skipped_footer=%d elapsed=%s\n",
		len(paths), rows, emitted, rejected, skipped, time.Since(startedAt).Round(time.Millisecond))
}

func formatCallCounts(entries []rbncache.CallEntry) string {
	if len(entries) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, fmt.Sprintf("%s:%d", entry.DXCall, entry.Spots))
	}
	return strings.Join(parts, ",")
}
