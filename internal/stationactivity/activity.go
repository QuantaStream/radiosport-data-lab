package stationactivity

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const (
	EventType          = "station_activity_5m_summary"
	DefaultSourceTable = "spots_flat"
	AllContinents      = "ALL"
)

type Observation struct {
	DXCall           string
	DXPrefix         string
	DXContinent      string
	Activity5MID     uint64
	Activity5MKey    string
	BucketKey        int
	SpottedAt        time.Time
	Band             string
	Mode             string
	SpotterCall      string
	SpotterPrefix    string
	SpotterContinent string
	SignalDB         int
}

type Options struct {
	SourceTable      string
	ComputedAt       time.Time
	EmitAllContinent bool
}

type Summary struct {
	SummaryID               uint64
	SummaryKey              string
	Activity5MID            uint64
	Activity5MKey           string
	BucketKey               int
	BucketStart             time.Time
	DXCall                  string
	DXPrefix                string
	DXContinent             string
	Band                    string
	Mode                    string
	SpotterContinent        string
	SpotCount               int
	DistinctSpotters        int
	DistinctSpotterPrefixes int
	AvgSignalDB             float64
	MinSignalDB             int
	MaxSignalDB             int
	P50SignalDB             float64
	P90SignalDB             float64
	ReachScore              float64
	SourceTable             string
	ComputedAt              time.Time
}

func BuildSummaries(observations []Observation, options Options) []Summary {
	if len(observations) == 0 {
		return nil
	}
	accumulator := NewAccumulator(options)
	for _, observation := range observations {
		accumulator.Add(observation)
	}
	return accumulator.Summaries()
}

type Accumulator struct {
	options Options
	groups  map[string]*builder
}

func NewAccumulator(options Options) *Accumulator {
	return &Accumulator{
		options: normalizeOptions(options),
		groups:  map[string]*builder{},
	}
}

func (a *Accumulator) Add(observation Observation) {
	if a == nil {
		return
	}
	observation = normalizeObservation(observation)
	if observation.DXCall == "" || observation.Activity5MID == 0 {
		return
	}
	addObservation(a.groups, observation, observation.SpotterContinent)
	if a.options.EmitAllContinent {
		addObservation(a.groups, observation, AllContinents)
	}
}

func (a *Accumulator) Summaries() []Summary {
	if a == nil || len(a.groups) == 0 {
		return nil
	}
	summaries := make([]Summary, 0, len(a.groups))
	for _, group := range a.groups {
		summaries = append(summaries, group.summary(a.options))
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SummaryKey < summaries[j].SummaryKey
	})
	return summaries
}

func addObservation(groups map[string]*builder, observation Observation, spotterContinent string) {
	key := SummaryKey(observation.DXCall, observation.Band, observation.Mode, observation.Activity5MKey, spotterContinent)
	group := groups[key]
	if group == nil {
		group = newBuilder(observation, spotterContinent, key)
		groups[key] = group
	}
	group.add(observation)
}

func SummaryKey(dxCall string, band string, mode string, activity5MKey string, spotterContinent string) string {
	return strings.Join([]string{
		normalizeKeyPart(dxCall),
		normalizeKeyPart(band),
		normalizeKeyPart(mode),
		normalizeKeyPart(activity5MKey),
		normalizeKeyPart(spotterContinent),
	}, "|")
}

func StableSummaryID(summaryKey string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.ToUpper(strings.TrimSpace(summaryKey))))
	id := h.Sum64() & ((uint64(1) << 63) - 1)
	if id == 0 {
		return 1
	}
	return id
}

type builder struct {
	key              string
	activity5MID     uint64
	activity5MKey    string
	bucketKey        int
	bucketStart      time.Time
	dxCall           string
	dxPrefix         string
	dxContinent      string
	band             string
	mode             string
	spotterContinent string
	spotCount        int
	signalSum        int
	minSignal        int
	maxSignal        int
	signals          []int
	spotters         map[string]struct{}
	spotterPrefixes  map[string]struct{}
}

func newBuilder(observation Observation, spotterContinent string, key string) *builder {
	return &builder{
		key:              key,
		activity5MID:     observation.Activity5MID,
		activity5MKey:    observation.Activity5MKey,
		bucketKey:        observation.BucketKey,
		bucketStart:      rbn.FiveMinuteBucketStartUTC(observation.SpottedAt),
		dxCall:           observation.DXCall,
		dxPrefix:         observation.DXPrefix,
		dxContinent:      observation.DXContinent,
		band:             observation.Band,
		mode:             observation.Mode,
		spotterContinent: spotterContinent,
		minSignal:        observation.SignalDB,
		maxSignal:        observation.SignalDB,
		spotters:         map[string]struct{}{},
		spotterPrefixes:  map[string]struct{}{},
	}
}

func (b *builder) add(observation Observation) {
	b.spotCount++
	b.signalSum += observation.SignalDB
	if observation.SignalDB < b.minSignal {
		b.minSignal = observation.SignalDB
	}
	if observation.SignalDB > b.maxSignal {
		b.maxSignal = observation.SignalDB
	}
	b.signals = append(b.signals, observation.SignalDB)
	if observation.SpotterCall != "" {
		b.spotters[observation.SpotterCall] = struct{}{}
	}
	if observation.SpotterPrefix != "" {
		b.spotterPrefixes[observation.SpotterPrefix] = struct{}{}
	}
}

func (b *builder) summary(options Options) Summary {
	sort.Ints(b.signals)
	avg := float64(b.signalSum) / float64(b.spotCount)
	reachScore := float64(len(b.spotters)) * math.Log1p(float64(len(b.spotterPrefixes))) * math.Max(avg, 1)
	return Summary{
		SummaryID:               StableSummaryID(b.key),
		SummaryKey:              b.key,
		Activity5MID:            b.activity5MID,
		Activity5MKey:           b.activity5MKey,
		BucketKey:               b.bucketKey,
		BucketStart:             b.bucketStart,
		DXCall:                  b.dxCall,
		DXPrefix:                b.dxPrefix,
		DXContinent:             b.dxContinent,
		Band:                    b.band,
		Mode:                    b.mode,
		SpotterContinent:        b.spotterContinent,
		SpotCount:               b.spotCount,
		DistinctSpotters:        len(b.spotters),
		DistinctSpotterPrefixes: len(b.spotterPrefixes),
		AvgSignalDB:             avg,
		MinSignalDB:             b.minSignal,
		MaxSignalDB:             b.maxSignal,
		P50SignalDB:             percentileNearestRank(b.signals, 0.50),
		P90SignalDB:             percentileNearestRank(b.signals, 0.90),
		ReachScore:              reachScore,
		SourceTable:             options.SourceTable,
		ComputedAt:              options.ComputedAt,
	}
}

func normalizeOptions(options Options) Options {
	if strings.TrimSpace(options.SourceTable) == "" {
		options.SourceTable = DefaultSourceTable
	}
	if options.ComputedAt.IsZero() {
		options.ComputedAt = time.Now().UTC()
	}
	return options
}

func normalizeObservation(observation Observation) Observation {
	if call, ok := rbn.NormalizeCallsign(observation.DXCall); ok {
		observation.DXCall = call
	} else {
		observation.DXCall = ""
	}
	if call, ok := rbn.NormalizeCallsign(observation.SpotterCall); ok {
		observation.SpotterCall = call
	} else {
		observation.SpotterCall = ""
	}
	observation.DXPrefix = normalizeCode(observation.DXPrefix)
	observation.DXContinent = normalizeCode(observation.DXContinent)
	observation.SpotterPrefix = normalizeCode(observation.SpotterPrefix)
	observation.SpotterContinent = normalizeCode(observation.SpotterContinent)
	observation.Band = normalizeCode(observation.Band)
	observation.Mode = normalizeCode(observation.Mode)
	if observation.Activity5MKey == "" && !observation.SpottedAt.IsZero() {
		observation.Activity5MKey = rbn.Activity5MKey(observation.DXCall, observation.Band, observation.Mode, observation.SpottedAt)
	}
	if observation.Activity5MID == 0 && observation.Activity5MKey != "" {
		observation.Activity5MID = rbn.Activity5MID(observation.Activity5MKey)
	}
	if observation.BucketKey == 0 && !observation.SpottedAt.IsZero() {
		observation.BucketKey = rbn.FiveMinuteBucketKeyUTC(observation.SpottedAt)
	}
	return observation
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

func normalizeCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		return rbn.UnknownValue
	}
	return value
}

func normalizeKeyPart(value string) string {
	value = normalizeCode(value)
	replacer := strings.NewReplacer(" ", "_", "\t", "_", "\n", "_", "\r", "_")
	return replacer.Replace(value)
}

func ValidateSummary(summary Summary) error {
	if summary.SummaryID == 0 {
		return fmt.Errorf("summary %q has zero id", summary.SummaryKey)
	}
	if summary.SummaryKey == "" {
		return fmt.Errorf("summary key is required")
	}
	if _, ok := rbn.NormalizeCallsign(summary.DXCall); !ok {
		return fmt.Errorf("invalid dx callsign %q", summary.DXCall)
	}
	if summary.Activity5MID == 0 {
		return fmt.Errorf("summary %q has zero activity id", summary.SummaryKey)
	}
	if summary.SpotCount <= 0 {
		return fmt.Errorf("summary %q has no spots", summary.SummaryKey)
	}
	return nil
}

func ParseUint64Signed(value int64) uint64 {
	if value < 0 {
		return 0
	}
	return uint64(value)
}

func FormatSummaryID(id uint64) string {
	return strconv.FormatUint(id, 10)
}
