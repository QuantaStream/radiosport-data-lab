package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

type reachKey struct {
	bucket, station, band string
}

type regionKey struct {
	reachKey
	continent string
}

type reachValue struct {
	spots      int
	skimmers   map[string]struct{}
	continents map[string]struct{}
	db         []int
}

type receiverValue struct {
	sum, count int
	continent  string
}

func main() {
	log.SetFlags(0)
	var callsArg, compareArg, outputDir, bandActivityPath string
	flag.StringVar(&callsArg, "calls", "EF8R,CQ9A,5J1DX", "comma-separated target calls")
	flag.StringVar(&compareArg, "compare", "EF8R,CQ9A", "two calls for matched-skimmer comparison")
	flag.StringVar(&outputDir, "output-dir", ".", "directory for CSV outputs")
	flag.StringVar(&bandActivityPath, "band-activity", "", "optional battle-report band-activity.csv for daily alignment")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("usage: cqww-rbn-report [flags] <RBN daily .zip or .csv>...")
	}

	calls := parseCalls(callsArg)
	compare := parseCalls(compareArg)
	if len(calls) == 0 || len(compare) != 2 {
		log.Fatal("-calls must be nonempty and -compare must contain exactly two calls")
	}
	targets := make(map[string]struct{}, len(calls))
	for _, call := range calls {
		targets[call] = struct{}{}
	}

	reach := make(map[reachKey]*reachValue)
	regionReach := make(map[regionKey]*reachValue)
	byReceiver := make(map[reachKey]map[string]*receiverValue)
	var matchedSpots int
	for _, path := range flag.Args() {
		reader, archiveDate, err := rbn.OpenArchiveFile(path)
		if err != nil {
			log.Fatal(err)
		}
		stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
			if _, ok := targets[spot.DXCall]; !ok {
				return nil
			}
			matchedSpots++
			key := reachKey{
				bucket:  rbn.FiveMinuteBucketStartUTC(spot.SpottedAt).Format(time.RFC3339),
				station: spot.DXCall,
				band:    spot.Band,
			}
			addReach(reach, key, spot)
			continent := spot.SpotterContinent
			if continent == "" {
				continent = rbn.UnknownValue
			}
			addRegionReach(regionReach, regionKey{reachKey: key, continent: continent}, spot)
			receivers := byReceiver[key]
			if receivers == nil {
				receivers = map[string]*receiverValue{}
				byReceiver[key] = receivers
			}
			rv := receivers[spot.SpotterCall]
			if rv == nil {
				rv = &receiverValue{}
				receivers[spot.SpotterCall] = rv
			}
			rv.sum += spot.SignalDB
			rv.count++
			rv.continent = spot.SpotterContinent
			return nil
		})
		_ = reader.Close()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s: %d rows, %d rejected", filepath.Base(path), stats.Rows, stats.RejectedRows)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := writeReach(filepath.Join(outputDir, "rbn-reach.csv"), reach); err != nil {
		log.Fatal(err)
	}
	if err := writeRegionReach(filepath.Join(outputDir, "rbn-reach-region.csv"), regionReach); err != nil {
		log.Fatal(err)
	}
	if err := writeRegionReachJSONL(filepath.Join(outputDir, "rbn-reach-region.jsonl"), regionReach); err != nil {
		log.Fatal(err)
	}
	if err := writeMatched(filepath.Join(outputDir, "rbn-matched-skimmers.csv"), compare, byReceiver); err != nil {
		log.Fatal(err)
	}
	if bandActivityPath != "" {
		qsos, err := readHighBandQSOs(bandActivityPath)
		if err != nil {
			log.Fatal(err)
		}
		if err := writeDailyHighBand(filepath.Join(outputDir, "rbn-high-band-daily.csv"), calls, qsos, reach, regionReach); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("wrote %d target spots to %s", matchedSpots, outputDir)
}

func writeRegionReachJSONL(path string, values map[regionKey]*reachValue) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(f)
	for _, key := range sortedRegionKeys(values) {
		v := values[key]
		data := map[string]any{
			"reach_id": stableReachID(key), "case_id": "cqww-cw-2025-soab-hp-battle",
			"bucket_start": key.bucket, "station_call": key.station, "band": key.band,
			"spotter_continent": key.continent, "spot_count": v.spots,
			"unique_skimmers":    len(v.skimmers),
			"median_snr_millidb": int(percentile(v.db, 0.5) * 1000),
			"p90_snr_millidb":    int(percentile(v.db, 0.9) * 1000),
		}
		if err := encoder.Encode(map[string]any{"type": "cqww_rbn_reach_bucket", "data": data}); err != nil {
			_ = f.Close()
			return err
		}
	}
	return f.Close()
}

func stableReachID(key regionKey) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("cqww-cw-2025-soab-hp-battle\x00" + key.bucket + "\x00" + key.station + "\x00" + key.band + "\x00" + key.continent))
	id := h.Sum64() & ((uint64(1) << 63) - 1)
	if id == 0 {
		return 1
	}
	return id
}

type dayStation struct{ day, station string }

func readHighBandQSOs(path string) (map[dayStation]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}
	for _, required := range []string{"bucket_start", "station", "band", "qsos"} {
		if _, ok := index[required]; !ok {
			return nil, fmt.Errorf("%s lacks %q column", path, required)
		}
	}
	out := map[dayStation]int{}
	for {
		record, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		band := record[index["band"]]
		if band != "10m" && band != "15m" {
			continue
		}
		value, err := strconv.Atoi(record[index["qsos"]])
		if err != nil {
			return nil, err
		}
		bucket := record[index["bucket_start"]]
		if len(bucket) < 10 {
			return nil, fmt.Errorf("invalid bucket_start %q", bucket)
		}
		key := dayStation{bucket[:10], record[index["station"]]}
		out[key] += value
	}
	return out, nil
}

func writeDailyHighBand(path string, calls []string, qsos map[dayStation]int, reach map[reachKey]*reachValue, regions map[regionKey]*reachValue) error {
	type regionTotals struct{ spots, skimmerBuckets int }
	active := map[dayStation]map[string]map[string]struct{}{}
	region := map[dayStation]map[string]*regionTotals{}
	days := map[string]struct{}{}
	for key := range reach {
		if key.band != "10m" && key.band != "15m" {
			continue
		}
		day := key.bucket[:10]
		ds := dayStation{day, key.station}
		days[day] = struct{}{}
		if active[ds] == nil {
			active[ds] = map[string]map[string]struct{}{}
		}
		if active[ds][key.bucket] == nil {
			active[ds][key.bucket] = map[string]struct{}{}
		}
		active[ds][key.bucket][key.band] = struct{}{}
	}
	for key, value := range regions {
		if key.band != "10m" && key.band != "15m" {
			continue
		}
		ds := dayStation{key.bucket[:10], key.station}
		if region[ds] == nil {
			region[ds] = map[string]*regionTotals{}
		}
		v := region[ds][key.continent]
		if v == nil {
			v = &regionTotals{}
			region[ds][key.continent] = v
		}
		v.spots += value.spots
		v.skimmerBuckets += len(value.skimmers)
	}
	orderedDays := make([]string, 0, len(days))
	for day := range days {
		orderedDays = append(orderedDays, day)
	}
	sort.Strings(orderedDays)
	header := []string{"day", "station", "high_band_qsos", "active_5m_intervals", "dual_high_band_5m_intervals", "eu_skimmer_buckets", "eu_spots", "na_skimmer_buckets", "na_spots"}
	return writeCSV(path, header, func(w *csv.Writer) error {
		for _, day := range orderedDays {
			for _, station := range calls {
				ds := dayStation{day, station}
				dual := 0
				for _, bands := range active[ds] {
					if len(bands) == 2 {
						dual++
					}
				}
				eu, na := region[ds]["EU"], region[ds]["NA"]
				if eu == nil {
					eu = &regionTotals{}
				}
				if na == nil {
					na = &regionTotals{}
				}
				record := []string{day, station, strconv.Itoa(qsos[ds]), strconv.Itoa(len(active[ds])), strconv.Itoa(dual), strconv.Itoa(eu.skimmerBuckets), strconv.Itoa(eu.spots), strconv.Itoa(na.skimmerBuckets), strconv.Itoa(na.spots)}
				if err := w.Write(record); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func addReach(values map[reachKey]*reachValue, key reachKey, spot rbn.Spot) {
	v := values[key]
	if v == nil {
		v = &reachValue{skimmers: map[string]struct{}{}, continents: map[string]struct{}{}}
		values[key] = v
	}
	v.spots++
	v.skimmers[spot.SpotterCall] = struct{}{}
	v.continents[spot.SpotterContinent] = struct{}{}
	v.db = append(v.db, spot.SignalDB)
}

func addRegionReach(values map[regionKey]*reachValue, key regionKey, spot rbn.Spot) {
	v := values[key]
	if v == nil {
		v = &reachValue{skimmers: map[string]struct{}{}, continents: map[string]struct{}{}}
		values[key] = v
	}
	v.spots++
	v.skimmers[spot.SpotterCall] = struct{}{}
	v.db = append(v.db, spot.SignalDB)
}

func parseCalls(value string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range strings.Split(value, ",") {
		call, ok := rbn.NormalizeCallsign(raw)
		if !ok {
			continue
		}
		if _, ok := seen[call]; ok {
			continue
		}
		seen[call] = struct{}{}
		out = append(out, call)
	}
	return out
}

func writeReach(path string, values map[reachKey]*reachValue) error {
	keys := sortedKeys(values)
	return writeCSV(path, []string{"bucket_start", "station", "band", "spot_count", "unique_skimmers", "spotter_continents", "median_snr_db", "p90_snr_db"}, func(w *csv.Writer) error {
		for _, key := range keys {
			v := values[key]
			if err := w.Write([]string{key.bucket, key.station, key.band, strconv.Itoa(v.spots), strconv.Itoa(len(v.skimmers)), strconv.Itoa(len(v.continents)), formatFloat(percentile(v.db, 0.5)), formatFloat(percentile(v.db, 0.9))}); err != nil {
				return err
			}
		}
		return nil
	})
}

func writeRegionReach(path string, values map[regionKey]*reachValue) error {
	keys := sortedRegionKeys(values)
	header := []string{"bucket_start", "station", "band", "spotter_continent", "spot_count", "unique_skimmers", "median_snr_db", "p90_snr_db"}
	return writeCSV(path, header, func(w *csv.Writer) error {
		for _, key := range keys {
			v := values[key]
			record := []string{key.bucket, key.station, key.band, key.continent, strconv.Itoa(v.spots), strconv.Itoa(len(v.skimmers)), formatFloat(percentile(v.db, 0.5)), formatFloat(percentile(v.db, 0.9))}
			if err := w.Write(record); err != nil {
				return err
			}
		}
		return nil
	})
}

func sortedRegionKeys(values map[regionKey]*reachValue) []regionKey {
	keys := make([]regionKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucket != keys[j].bucket {
			return keys[i].bucket < keys[j].bucket
		}
		if keys[i].station != keys[j].station {
			return keys[i].station < keys[j].station
		}
		if keys[i].band != keys[j].band {
			return keys[i].band < keys[j].band
		}
		return keys[i].continent < keys[j].continent
	})
	return keys
}

func writeMatched(path string, calls []string, values map[reachKey]map[string]*receiverValue) error {
	type pairKey struct{ bucket, band string }
	pairs := map[pairKey]struct{}{}
	for key := range values {
		pairs[pairKey{key.bucket, key.band}] = struct{}{}
	}
	keys := make([]pairKey, 0, len(pairs))
	for key := range pairs {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucket != keys[j].bucket {
			return keys[i].bucket < keys[j].bucket
		}
		return keys[i].band < keys[j].band
	})
	header := []string{"bucket_start", "band", "spotter_continent", "station_a", "station_b", "matched_skimmers", "median_snr_delta_a_minus_b_db", "mean_snr_delta_a_minus_b_db"}
	return writeCSV(path, header, func(w *csv.Writer) error {
		for _, key := range keys {
			a := values[reachKey{key.bucket, calls[0], key.band}]
			b := values[reachKey{key.bucket, calls[1], key.band}]
			deltasByContinent := map[string][]int{"ALL": {}}
			sumsByContinent := map[string]float64{"ALL": 0}
			for skimmer, av := range a {
				bv, ok := b[skimmer]
				if !ok || av.continent != bv.continent {
					continue
				}
				delta := float64(av.sum)/float64(av.count) - float64(bv.sum)/float64(bv.count)
				continent := av.continent
				if continent == "" {
					continent = rbn.UnknownValue
				}
				deltasByContinent["ALL"] = append(deltasByContinent["ALL"], int(delta*1000))
				sumsByContinent["ALL"] += delta
				deltasByContinent[continent] = append(deltasByContinent[continent], int(delta*1000))
				sumsByContinent[continent] += delta
			}
			continents := make([]string, 0, len(deltasByContinent))
			for continent, deltas := range deltasByContinent {
				if len(deltas) > 0 {
					continents = append(continents, continent)
				}
			}
			sort.Strings(continents)
			for _, continent := range continents {
				deltas := deltasByContinent[continent]
				record := []string{key.bucket, key.band, continent, calls[0], calls[1], strconv.Itoa(len(deltas)), formatFloat(percentile(deltas, 0.5) / 1000), formatFloat(sumsByContinent[continent] / float64(len(deltas)))}
				if err := w.Write(record); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func sortedKeys(values map[reachKey]*reachValue) []reachKey {
	keys := make([]reachKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucket != keys[j].bucket {
			return keys[i].bucket < keys[j].bucket
		}
		if keys[i].station != keys[j].station {
			return keys[i].station < keys[j].station
		}
		return keys[i].band < keys[j].band
	})
	return keys
}

func percentile(values []int, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]int(nil), values...)
	sort.Ints(copyValues)
	position := q * float64(len(copyValues)-1)
	lo, hi := int(position), int(position+0.999999999)
	if lo == hi {
		return float64(copyValues[lo])
	}
	fraction := position - float64(lo)
	return float64(copyValues[lo])*(1-fraction) + float64(copyValues[hi])*fraction
}

func writeCSV(path string, header []string, emit func(*csv.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		_ = f.Close()
		return err
	}
	if err := emit(w); err != nil {
		_ = f.Close()
		return err
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func formatFloat(value float64) string { return fmt.Sprintf("%.2f", value) }
