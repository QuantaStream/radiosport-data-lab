package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(0)

	var dxCalls string
	var includeZero bool
	flag.StringVar(&dxCalls, "dx-calls", "", "Comma-separated DX callsigns to count.")
	flag.BoolVar(&includeZero, "include-zero", false, "Print archives with zero target-call matches.")
	flag.Parse()

	targets := parseCalls(dxCalls)
	if len(targets) == 0 {
		log.Fatal("usage: rbn-archive-scan -dx-calls CALL[,CALL...] <RBN daily .zip or .csv>...")
	}
	if flag.NArg() == 0 {
		log.Fatal("at least one RBN daily .zip or .csv file is required")
	}

	fmt.Print("day,rows,rejected,skipped_footer")
	for _, target := range targets {
		fmt.Printf(",%s", target)
	}
	fmt.Println(",total")

	totals := make(map[string]int, len(targets))
	var grandTotal int
	for _, path := range flag.Args() {
		result, err := scanArchive(path, targets)
		if err != nil {
			log.Fatalf("%s: %v", path, err)
		}
		if result.Total == 0 && !includeZero {
			continue
		}
		fmt.Printf("%s,%d,%d,%d", result.Day, result.Rows, result.RejectedRows, result.SkippedFooter)
		for _, target := range targets {
			fmt.Printf(",%d", result.Counts[target])
			totals[target] += result.Counts[target]
		}
		grandTotal += result.Total
		fmt.Printf(",%d\n", result.Total)
	}

	fmt.Printf("TOTAL,,,,")
	for i, target := range targets {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Print(totals[target])
	}
	fmt.Printf(",%d\n", grandTotal)
}

type scanResult struct {
	Day           string
	Rows          int
	RejectedRows  int
	SkippedFooter int
	Counts        map[string]int
	Total         int
}

func scanArchive(path string, targets []string) (scanResult, error) {
	reader, archiveDate, err := rbn.OpenArchiveFile(path)
	if err != nil {
		return scanResult{}, err
	}
	defer reader.Close()

	targetSet := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		targetSet[target] = struct{}{}
	}
	result := scanResult{
		Day:    archiveDay(path),
		Counts: make(map[string]int, len(targets)),
	}
	for _, target := range targets {
		result.Counts[target] = 0
	}

	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		if _, ok := targetSet[spot.DXCall]; ok {
			result.Counts[spot.DXCall]++
			result.Total++
		}
		return nil
	})
	if err != nil {
		return scanResult{}, err
	}
	result.Rows = stats.Rows
	result.RejectedRows = stats.RejectedRows
	result.SkippedFooter = stats.SkippedFooter
	return result, nil
}

func parseCalls(value string) []string {
	seen := map[string]struct{}{}
	var calls []string
	for _, part := range strings.Split(value, ",") {
		call, ok := rbn.NormalizeCallsign(part)
		if !ok {
			continue
		}
		if _, exists := seen[call]; exists {
			continue
		}
		seen[call] = struct{}{}
		calls = append(calls, call)
	}
	sort.Strings(calls)
	return calls
}

func archiveDay(path string) string {
	date := rbn.ArchiveDateFromName(path)
	if date.IsZero() {
		return strings.TrimSuffix(strings.TrimSuffix(osFileBase(path), ".zip"), ".csv")
	}
	return fmt.Sprintf("%04d%02d%02d", date.Year(), date.Month(), date.Day())
}

func osFileBase(path string) string {
	path = strings.TrimRight(path, string(os.PathSeparator))
	parts := strings.FieldsFunc(path, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}
