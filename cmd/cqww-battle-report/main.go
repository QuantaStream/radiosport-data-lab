package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/cqwwscore"
)

type stationRun struct {
	Log     cabrillo.Log
	Summary cqwwscore.Summary
	Buckets []cqwwscore.Bucket
	Bands   []cqwwscore.BandBucket
}

func main() {
	ctyPath := flag.String("cty-dat", "data/cty/cty.dat", "contest-date CTY file")
	outDir := flag.String("out-dir", "", "output directory")
	startText := flag.String("start", "2025-11-29T00:00:00Z", "contest start, RFC3339")
	endText := flag.String("end", "2025-12-01T00:00:00Z", "contest end, RFC3339")
	interval := flag.Duration("interval", 5*time.Minute, "timeline interval")
	flag.Parse()
	if flag.NArg() == 0 || *outDir == "" {
		fatalf("usage: cqww-battle-report -out-dir DIR [flags] log...")
	}
	start := mustTime(*startText)
	end := mustTime(*endText)
	db, err := callsign.LoadFile(*ctyPath)
	if err != nil {
		fatalf("load CTY: %v", err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("create output: %v", err)
	}

	runs := make([]stationRun, 0, flag.NArg())
	for _, path := range flag.Args() {
		f, err := os.Open(path)
		if err != nil {
			fatalf("open %s: %v", path, err)
		}
		contestLog, qsos, _, parseErr := cabrillo.Parse(f, cabrillo.ParseOptions{SourceFile: filepath.Base(path), CallsignDB: db})
		closeErr := f.Close()
		if parseErr != nil {
			fatalf("parse %s: %v", path, parseErr)
		}
		if closeErr != nil {
			fatalf("close %s: %v", path, closeErr)
		}
		states, summary, err := cqwwscore.Reconstruct(contestLog, qsos, db)
		if err != nil {
			fatalf("score %s: %v", path, err)
		}
		buckets, bands, err := cqwwscore.BuildBuckets(states, start, end, *interval)
		if err != nil {
			fatalf("timeline %s: %v", path, err)
		}
		runs = append(runs, stationRun{Log: contestLog, Summary: summary, Buckets: buckets, Bands: bands})
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].Log.StationCall < runs[j].Log.StationCall })
	if err := writeSummaries(filepath.Join(*outDir, "summaries.csv"), runs); err != nil {
		fatalf("summaries: %v", err)
	}
	if err := writeTimeline(filepath.Join(*outDir, "score-timeline.csv"), runs); err != nil {
		fatalf("timeline: %v", err)
	}
	if err := writeTimelineJSONL(filepath.Join(*outDir, "battle-timeline.jsonl"), runs); err != nil {
		fatalf("timeline JSONL: %v", err)
	}
	if err := writeBands(filepath.Join(*outDir, "band-activity.csv"), runs); err != nil {
		fatalf("bands: %v", err)
	}
	if err := writeLeadChanges(filepath.Join(*outDir, "lead-changes.csv"), runs); err != nil {
		fatalf("leads: %v", err)
	}
	if err := writeCheckpoints(filepath.Join(*outDir, "checkpoints.csv"), runs, start); err != nil {
		fatalf("checkpoints: %v", err)
	}
	if err := writeThreeHourCheckpoints(filepath.Join(*outDir, "three-hour-checkpoints.csv"), runs, start); err != nil {
		fatalf("three-hour checkpoints: %v", err)
	}
}

func writeTimelineJSONL(path string, runs []stationRun) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	if len(runs) > 0 {
		for i := range runs[0].Buckets {
			leader, maxScore, margin := leaderAt(runs, i)
			for _, r := range runs {
				b := r.Buckets[i]
				leaderMargin := int64(0)
				if b.StationCall == leader {
					leaderMargin = margin
				}
				data := map[string]any{
					"bucket_id": stableBucketID("cqww-cw-2025-soab-hp-battle", b.StationCall, b.BucketStart),
					"case_id":   "cqww-cw-2025-soab-hp-battle", "scoring_model": "cty-3537",
					"bucket_start": rfc3339(b.BucketStart), "bucket_end": rfc3339(b.BucketEnd),
					"station_call": b.StationCall, "leader_call": leader,
					"score_behind_leader": maxScore - b.CumulativeScore, "leader_margin": leaderMargin,
					"bucket_qsos": b.BucketQSOs, "bucket_counted_qsos": b.BucketCountedQSOs,
					"bucket_duplicates": b.BucketDuplicates, "bucket_points": b.BucketPoints,
					"bucket_countries": b.BucketCountries, "bucket_zones": b.BucketZones,
					"bucket_10m": b.Bucket10M, "bucket_15m": b.Bucket15M, "bucket_20m": b.Bucket20M,
					"bucket_40m": b.Bucket40M, "bucket_80m": b.Bucket80M, "bucket_160m": b.Bucket160M,
					"cumulative_qsos": b.CumulativeQSOs, "cumulative_counted_qsos": b.CumulativeCounted,
					"cumulative_duplicates": b.CumulativeDuplicate, "cumulative_points": b.CumulativePoints,
					"cumulative_countries": b.CumulativeCountries, "cumulative_zones": b.CumulativeZones,
					"cumulative_multipliers": b.CumulativeMultipliers, "cumulative_score": b.CumulativeScore,
				}
				if err := encoder.Encode(map[string]any{"type": "cqww_battle_bucket", "data": data}); err != nil {
					_ = f.Close()
					return err
				}
			}
		}
	}
	return f.Close()
}

func stableBucketID(caseID, station string, at time.Time) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(caseID + "\x00" + station + "\x00" + rfc3339(at)))
	id := h.Sum64() & ((uint64(1) << 63) - 1)
	if id == 0 {
		return 1
	}
	return id
}

func writeSummaries(path string, runs []stationRun) error {
	return writeCSV(path, []string{"station", "submitted_qsos", "counted_qsos", "duplicates", "unresolved", "qso_points", "countries", "zones", "multipliers", "reconstructed_score", "claimed_score", "score_delta"}, func(w *csv.Writer) error {
		for _, r := range runs {
			s := r.Summary
			if err := w.Write([]string{s.StationCall, itoa(s.SubmittedQSOs), itoa(s.CountedQSOs), itoa(s.Duplicates), itoa(s.Unresolved), itoa(s.QSOPoints), itoa(s.CountryMultipliers), itoa(s.ZoneMultipliers), itoa(s.Multipliers), i64toa(s.Score), i64toa(r.Log.ClaimedScore), i64toa(s.Score - r.Log.ClaimedScore)}); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeTimeline(path string, runs []stationRun) error {
	header := []string{"bucket_start", "bucket_end", "elapsed_hours", "station", "leader", "score_behind_leader", "leader_margin", "bucket_qsos", "bucket_counted_qsos", "bucket_duplicates", "bucket_points", "bucket_countries", "bucket_zones", "bucket_10m", "bucket_15m", "bucket_20m", "bucket_40m", "bucket_80m", "bucket_160m", "cumulative_qsos", "cumulative_counted_qsos", "cumulative_duplicates", "cumulative_points", "cumulative_countries", "cumulative_zones", "cumulative_multipliers", "cumulative_score"}
	return writeCSV(path, header, func(w *csv.Writer) error {
		if len(runs) == 0 {
			return nil
		}
		for i := range runs[0].Buckets {
			leader, maxScore, margin := leaderAt(runs, i)
			for _, r := range runs {
				b := r.Buckets[i]
				row := []string{rfc3339(b.BucketStart), rfc3339(b.BucketEnd), fmt.Sprintf("%.4f", b.BucketEnd.Sub(runs[0].Buckets[0].BucketStart).Hours()), b.StationCall, leader, i64toa(maxScore - b.CumulativeScore), "0", itoa(b.BucketQSOs), itoa(b.BucketCountedQSOs), itoa(b.BucketDuplicates), itoa(b.BucketPoints), itoa(b.BucketCountries), itoa(b.BucketZones), itoa(b.Bucket10M), itoa(b.Bucket15M), itoa(b.Bucket20M), itoa(b.Bucket40M), itoa(b.Bucket80M), itoa(b.Bucket160M), itoa(b.CumulativeQSOs), itoa(b.CumulativeCounted), itoa(b.CumulativeDuplicate), itoa(b.CumulativePoints), itoa(b.CumulativeCountries), itoa(b.CumulativeZones), itoa(b.CumulativeMultipliers), i64toa(b.CumulativeScore)}
				if b.StationCall == leader {
					row[6] = i64toa(margin)
				}
				if err := w.Write(row); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func writeBands(path string, runs []stationRun) error {
	return writeCSV(path, []string{"bucket_start", "bucket_end", "station", "band", "qsos", "counted_qsos", "duplicates", "unresolved", "points", "new_countries", "new_zones"}, func(w *csv.Writer) error {
		for _, r := range runs {
			for _, b := range r.Bands {
				if err := w.Write([]string{rfc3339(b.BucketStart), rfc3339(b.BucketEnd), b.StationCall, b.Band, itoa(b.QSOs), itoa(b.CountedQSOs), itoa(b.Duplicates), itoa(b.Unresolved), itoa(b.Points), itoa(b.NewCountries), itoa(b.NewZones)}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func writeLeadChanges(path string, runs []stationRun) error {
	return writeCSV(path, []string{"at", "from_leader", "to_leader", "leader_score", "margin"}, func(w *csv.Writer) error {
		if len(runs) == 0 {
			return nil
		}
		previous := ""
		for i := range runs[0].Buckets {
			leader, score, margin := leaderAt(runs, i)
			if score > 0 && leader != previous {
				if err := w.Write([]string{rfc3339(runs[0].Buckets[i].BucketEnd), previous, leader, i64toa(score), i64toa(margin)}); err != nil {
					return err
				}
				previous = leader
			}
		}
		return nil
	})
}

func writeCheckpoints(path string, runs []stationRun, start time.Time) error {
	return writeCheckpointHours(path, runs, start, []int{12, 24, 36, 48})
}

func writeThreeHourCheckpoints(path string, runs []stationRun, start time.Time) error {
	hours := make([]int, 0, 16)
	for hour := 3; hour <= 48; hour += 3 {
		hours = append(hours, hour)
	}
	return writeCheckpointHours(path, runs, start, hours)
}

func writeCheckpointHours(path string, runs []stationRun, start time.Time, hoursList []int) error {
	return writeCSV(path, []string{"elapsed_hours", "at", "rank", "station", "score", "score_behind", "qsos", "points", "countries", "zones", "multipliers"}, func(w *csv.Writer) error {
		for _, hours := range hoursList {
			at := start.Add(time.Duration(hours) * time.Hour)
			type entry struct {
				call string
				b    cqwwscore.Bucket
			}
			entries := make([]entry, 0, len(runs))
			for _, r := range runs {
				for _, b := range r.Buckets {
					if b.BucketEnd.Equal(at) {
						entries = append(entries, entry{r.Log.StationCall, b})
						break
					}
				}
			}
			sort.Slice(entries, func(i, j int) bool {
				if entries[i].b.CumulativeScore == entries[j].b.CumulativeScore {
					return entries[i].call < entries[j].call
				}
				return entries[i].b.CumulativeScore > entries[j].b.CumulativeScore
			})
			var top int64
			if len(entries) > 0 {
				top = entries[0].b.CumulativeScore
			}
			for i, e := range entries {
				b := e.b
				if err := w.Write([]string{itoa(hours), rfc3339(at), itoa(i + 1), e.call, i64toa(b.CumulativeScore), i64toa(top - b.CumulativeScore), itoa(b.CumulativeCounted), itoa(b.CumulativePoints), itoa(b.CumulativeCountries), itoa(b.CumulativeZones), itoa(b.CumulativeMultipliers)}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func leaderAt(runs []stationRun, index int) (string, int64, int64) {
	type entry struct {
		call  string
		score int64
	}
	entries := make([]entry, 0, len(runs))
	for _, r := range runs {
		entries = append(entries, entry{r.Log.StationCall, r.Buckets[index].CumulativeScore})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].score == entries[j].score {
			return entries[i].call < entries[j].call
		}
		return entries[i].score > entries[j].score
	})
	if len(entries) == 0 {
		return "", 0, 0
	}
	margin := entries[0].score
	if len(entries) > 1 {
		margin -= entries[1].score
	}
	return entries[0].call, entries[0].score, margin
}

func writeCSV(path string, header []string, rows func(*csv.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err == nil {
		err = rows(w)
	}
	w.Flush()
	if err == nil {
		err = w.Error()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func mustTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		fatalf("time %q: %v", value, err)
	}
	return t
}
func fatalf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
func itoa(v int) string                 { return strconv.Itoa(v) }
func i64toa(v int64) string             { return strconv.FormatInt(v, 10) }
func rfc3339(v time.Time) string        { return v.UTC().Format(time.RFC3339) }
