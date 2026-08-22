package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatalf("usage: rbn-inspect <RBN daily .zip or .csv>")
	}

	reader, archiveDate, err := rbn.OpenArchiveFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()

	profile := newProfile()
	stats, err := rbn.ReadArchiveCSVWithDate(reader, archiveDate, func(spot rbn.Spot) error {
		profile.observe(spot)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("rows=%d rejected=%d skipped_footer=%d\n", stats.Rows, stats.RejectedRows, stats.SkippedFooter)
	fmt.Printf("spotter_calls=%d dx_calls=%d\n", len(profile.spotterCalls), len(profile.dxCalls))
	fmt.Printf("frequency_hz_min=%d frequency_hz_max=%d signal_db_min=%d signal_db_max=%d speed_wpm_min=%d speed_wpm_max=%d\n",
		profile.minFrequencyHz, profile.maxFrequencyHz, profile.minSignalDB, profile.maxSignalDB, profile.minSpeedWPM, profile.maxSpeedWPM)
	printTop("bands", profile.bands, 12)
	printTop("modes", profile.modes, 12)
	printTop("tx_modes", profile.txModes, 12)
	printTop("spotter_continents", profile.spotterContinents, 12)
	printTop("dx_continents", profile.dxContinents, 12)
}

type profile struct {
	spotterCalls      map[string]struct{}
	dxCalls           map[string]struct{}
	bands             map[string]int
	modes             map[string]int
	txModes           map[string]int
	spotterContinents map[string]int
	dxContinents      map[string]int
	minFrequencyHz    int64
	maxFrequencyHz    int64
	minSignalDB       int
	maxSignalDB       int
	minSpeedWPM       int
	maxSpeedWPM       int
	seen              bool
}

func newProfile() *profile {
	return &profile{
		spotterCalls:      map[string]struct{}{},
		dxCalls:           map[string]struct{}{},
		bands:             map[string]int{},
		modes:             map[string]int{},
		txModes:           map[string]int{},
		spotterContinents: map[string]int{},
		dxContinents:      map[string]int{},
	}
}

func (p *profile) observe(spot rbn.Spot) {
	p.spotterCalls[spot.SpotterCall] = struct{}{}
	p.dxCalls[spot.DXCall] = struct{}{}
	p.bands[spot.Band]++
	p.modes[spot.Mode]++
	p.txModes[spot.TransmitMode]++
	p.spotterContinents[spot.SpotterContinent]++
	p.dxContinents[spot.DXContinent]++
	if !p.seen {
		p.minFrequencyHz, p.maxFrequencyHz = spot.FrequencyHz, spot.FrequencyHz
		p.minSignalDB, p.maxSignalDB = spot.SignalDB, spot.SignalDB
		p.minSpeedWPM, p.maxSpeedWPM = spot.SpeedWPM, spot.SpeedWPM
		p.seen = true
		return
	}
	p.minFrequencyHz = min(p.minFrequencyHz, spot.FrequencyHz)
	p.maxFrequencyHz = max(p.maxFrequencyHz, spot.FrequencyHz)
	p.minSignalDB = min(p.minSignalDB, spot.SignalDB)
	p.maxSignalDB = max(p.maxSignalDB, spot.SignalDB)
	p.minSpeedWPM = min(p.minSpeedWPM, spot.SpeedWPM)
	p.maxSpeedWPM = max(p.maxSpeedWPM, spot.SpeedWPM)
}

func printTop(label string, counts map[string]int, limit int) {
	items := make([]item, 0, len(counts))
	for key, count := range counts {
		items = append(items, item{key: key, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].key < items[j].key
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	fmt.Printf("%s=", label)
	for i, item := range items {
		if i > 0 {
			fmt.Print(",")
		}
		fmt.Printf("%s:%d", item.key, item.count)
	}
	fmt.Println()
}

type item struct {
	key   string
	count int
}
