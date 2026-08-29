package spotterprofile

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/geo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const (
	NodeEventType     = "rbn_spotter_node"
	ProfileEventType  = "spotter_profile"
	SnapshotEventType = "spotter_profile_snapshot"

	DefaultProfileKind = "contest"
	DefaultSourceTable = "spots_flat"
	AllBands           = "ALL"
	AllModes           = "ALL"
)

type Observation struct {
	SpotterCall          string
	SpotterPrefix        string
	SpotterContinent     string
	SpotterCountry       string
	SpotterCQZone        int
	SpotterITUZone       int
	SpotterLatitude      float64
	SpotterLongitude     float64
	SpotterGeoSource     string
	SpotterGeoConfidence string
	DXCall               string
	DXPrefix             string
	SignalDB             int
	SpottedAt            time.Time
}

type Options struct {
	ProfileKind  string
	SourceTable  string
	WindowStart  time.Time
	WindowEnd    time.Time
	ComputedAt   time.Time
	MinGoodSpots int
	MinGoodHours int
	ProfileBand  string
	ProfileMode  string
}

type Summary struct {
	ProfileID             string
	SpotterCall           string
	SpotterPrefix         string
	SpotterContinent      string
	CountryName           string
	CQZone                int
	ITUZone               int
	Latitude              float64
	Longitude             float64
	GeoSource             string
	GeoConfidence         string
	ProfileKind           string
	SourceTable           string
	WindowStart           time.Time
	WindowEnd             time.Time
	Band                  string
	Mode                  string
	TotalSpots            int
	ActiveDays            int
	ActiveHours           int
	DistinctDXCalls       int
	DistinctDXPrefixes    int
	AvgSignalDB           float64
	MinSignalDB           int
	MaxSignalDB           int
	P50SignalDB           float64
	P90SignalDB           float64
	SpotterWeight         float64
	NormalizationOffsetDB float64
	ProfileQuality        string
	ComputedAt            time.Time
}

func BuildSummaries(observations []Observation, options Options) []Summary {
	if len(observations) == 0 {
		return nil
	}
	options = normalizeOptions(options)

	groups := map[string]*builder{}
	for _, observation := range observations {
		call, ok := rbn.NormalizeCallsign(observation.SpotterCall)
		if !ok {
			continue
		}
		observation.SpotterCall = call
		key := call
		group := groups[key]
		if group == nil {
			group = newBuilder(observation)
			groups[key] = group
		}
		group.add(observation)
	}

	summaries := make([]Summary, 0, len(groups))
	for _, group := range groups {
		summaries = append(summaries, group.summary(options))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SpotterCall < summaries[j].SpotterCall
	})
	return summaries
}

func NodeEvents(summaries []Summary) []interface{} {
	events := make([]interface{}, 0, len(summaries))
	for _, summary := range summaries {
		events = append(events, NewNodeEvent(summary))
	}
	return events
}

func SnapshotEvents(summaries []Summary) []interface{} {
	events := make([]interface{}, 0, len(summaries))
	for _, summary := range summaries {
		events = append(events, NewSnapshotEvent(summary))
	}
	return events
}

func ProfileEvents(summaries []Summary) []interface{} {
	events := make([]interface{}, 0, len(summaries))
	for _, summary := range summaries {
		events = append(events, NewProfileEvent(summary))
	}
	return events
}

func Events(summaries []Summary) []interface{} {
	events := make([]interface{}, 0, len(summaries)*3)
	events = append(events, NodeEvents(summaries)...)
	events = append(events, SnapshotEvents(summaries)...)
	events = append(events, ProfileEvents(summaries)...)
	return events
}

type builder struct {
	spotterCall      string
	spotterPrefix    string
	spotterContinent string
	countryName      string
	cqZone           int
	ituZone          int
	latitude         float64
	longitude        float64
	geoSource        string
	geoConfidence    string
	totalSpots       int
	signalSum        int
	minSignal        int
	maxSignal        int
	signals          []int
	activeDays       map[string]struct{}
	activeHours      map[string]struct{}
	dxCalls          map[string]struct{}
	dxPrefixes       map[string]struct{}
}

func newBuilder(observation Observation) *builder {
	prefix := strings.TrimSpace(observation.SpotterPrefix)
	if prefix == "" {
		prefix = rbn.UnknownValue
	}
	continent := strings.TrimSpace(observation.SpotterContinent)
	if continent == "" {
		continent = rbn.UnknownValue
	}
	countryName := strings.TrimSpace(observation.SpotterCountry)
	if countryName == "" {
		countryName = rbn.UnknownValue
	}
	location := geo.Unknown()
	geoSource := strings.TrimSpace(observation.SpotterGeoSource)
	if geoSource == "" {
		geoSource = location.Source
	}
	geoConfidence := strings.TrimSpace(observation.SpotterGeoConfidence)
	if geoConfidence == "" {
		geoConfidence = location.Confidence
	}
	return &builder{
		spotterCall:      observation.SpotterCall,
		spotterPrefix:    strings.ToUpper(prefix),
		spotterContinent: strings.ToUpper(continent),
		countryName:      countryName,
		cqZone:           observation.SpotterCQZone,
		ituZone:          observation.SpotterITUZone,
		latitude:         observation.SpotterLatitude,
		longitude:        observation.SpotterLongitude,
		geoSource:        strings.ToUpper(geoSource),
		geoConfidence:    strings.ToUpper(geoConfidence),
		minSignal:        observation.SignalDB,
		maxSignal:        observation.SignalDB,
		activeDays:       map[string]struct{}{},
		activeHours:      map[string]struct{}{},
		dxCalls:          map[string]struct{}{},
		dxPrefixes:       map[string]struct{}{},
	}
}

func (b *builder) add(observation Observation) {
	b.applyGeo(observation)
	b.totalSpots++
	b.signalSum += observation.SignalDB
	if observation.SignalDB < b.minSignal {
		b.minSignal = observation.SignalDB
	}
	if observation.SignalDB > b.maxSignal {
		b.maxSignal = observation.SignalDB
	}
	b.signals = append(b.signals, observation.SignalDB)
	if !observation.SpottedAt.IsZero() {
		utc := observation.SpottedAt.UTC()
		b.activeDays[utc.Format("2006-01-02")] = struct{}{}
		b.activeHours[utc.Format("2006-01-02T15")] = struct{}{}
	}
	if call := strings.TrimSpace(observation.DXCall); call != "" {
		b.dxCalls[strings.ToUpper(call)] = struct{}{}
	}
	if prefix := strings.TrimSpace(observation.DXPrefix); prefix != "" {
		b.dxPrefixes[strings.ToUpper(prefix)] = struct{}{}
	}
}

func (b *builder) applyGeo(observation Observation) {
	if strings.TrimSpace(b.countryName) == "" || b.countryName == rbn.UnknownValue {
		if country := strings.TrimSpace(observation.SpotterCountry); country != "" {
			b.countryName = country
		}
	}
	if b.cqZone == 0 {
		b.cqZone = observation.SpotterCQZone
	}
	if b.ituZone == 0 {
		b.ituZone = observation.SpotterITUZone
	}
	if b.geoSource == "" || b.geoSource == geo.SourceUnknown {
		source := strings.ToUpper(strings.TrimSpace(observation.SpotterGeoSource))
		confidence := strings.ToUpper(strings.TrimSpace(observation.SpotterGeoConfidence))
		if source != "" && source != geo.SourceUnknown {
			b.latitude = observation.SpotterLatitude
			b.longitude = observation.SpotterLongitude
			b.geoSource = source
			b.geoConfidence = confidence
		}
	}
}

func (b *builder) summary(options Options) Summary {
	sort.Ints(b.signals)
	avg := float64(b.signalSum) / float64(b.totalSpots)
	weight := 0.0
	if b.totalSpots > 0 {
		weight = 1 / math.Sqrt(float64(b.totalSpots))
	}
	quality := "sparse"
	if b.totalSpots >= options.MinGoodSpots && len(b.activeHours) >= options.MinGoodHours {
		quality = "good"
	}
	summary := Summary{
		SpotterCall:           b.spotterCall,
		SpotterPrefix:         b.spotterPrefix,
		SpotterContinent:      b.spotterContinent,
		CountryName:           b.countryName,
		CQZone:                b.cqZone,
		ITUZone:               b.ituZone,
		Latitude:              b.latitude,
		Longitude:             b.longitude,
		GeoSource:             b.geoSource,
		GeoConfidence:         b.geoConfidence,
		ProfileKind:           options.ProfileKind,
		SourceTable:           options.SourceTable,
		WindowStart:           options.WindowStart,
		WindowEnd:             options.WindowEnd,
		Band:                  options.ProfileBand,
		Mode:                  options.ProfileMode,
		TotalSpots:            b.totalSpots,
		ActiveDays:            len(b.activeDays),
		ActiveHours:           len(b.activeHours),
		DistinctDXCalls:       len(b.dxCalls),
		DistinctDXPrefixes:    len(b.dxPrefixes),
		AvgSignalDB:           avg,
		MinSignalDB:           b.minSignal,
		MaxSignalDB:           b.maxSignal,
		P50SignalDB:           percentileNearestRank(b.signals, 0.50),
		P90SignalDB:           percentileNearestRank(b.signals, 0.90),
		SpotterWeight:         weight,
		NormalizationOffsetDB: avg,
		ProfileQuality:        quality,
		ComputedAt:            options.ComputedAt,
	}
	summary.ProfileID = StableProfileID(summary)
	return summary
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.ProfileKind) == "" {
		options.ProfileKind = DefaultProfileKind
	}
	if strings.TrimSpace(options.SourceTable) == "" {
		options.SourceTable = DefaultSourceTable
	}
	if strings.TrimSpace(options.ProfileBand) == "" {
		options.ProfileBand = AllBands
	}
	if strings.TrimSpace(options.ProfileMode) == "" {
		options.ProfileMode = AllModes
	}
	if options.ComputedAt.IsZero() {
		options.ComputedAt = time.Now().UTC()
	}
	if options.MinGoodSpots <= 0 {
		options.MinGoodSpots = 25
	}
	if options.MinGoodHours <= 0 {
		options.MinGoodHours = 2
	}
	return options
}

func percentileNearestRank(sorted []int, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return float64(sorted[0])
	}
	if p >= 1 {
		return float64(sorted[len(sorted)-1])
	}
	index := int(math.Ceil(p*float64(len(sorted)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return float64(sorted[index])
}

func StableProfileID(summary Summary) string {
	parts := []string{
		sanitizeKeyPart(summary.ProfileKind),
		sanitizeKeyPart(summary.WindowStart.UTC().Format("20060102T150405Z")),
		sanitizeKeyPart(summary.WindowEnd.UTC().Format("20060102T150405Z")),
		sanitizeKeyPart(summary.Band),
		sanitizeKeyPart(summary.Mode),
		sanitizeKeyPart(summary.SpotterCall),
	}
	return strings.Join(parts, "|")
}

func sanitizeKeyPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "UNKNOWN"
	}
	value = strings.ToUpper(value)
	replacer := strings.NewReplacer(" ", "_", "\t", "_", "\n", "_", "\r", "_")
	return replacer.Replace(value)
}

func ValidateSummary(summary Summary) error {
	if _, ok := rbn.NormalizeCallsign(summary.SpotterCall); !ok {
		return fmt.Errorf("invalid spotter callsign %q", summary.SpotterCall)
	}
	if summary.TotalSpots <= 0 {
		return fmt.Errorf("summary for %s has no spots", summary.SpotterCall)
	}
	return nil
}
