package contestmatch

import (
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const (
	EventType      = "contest_spot_match"
	KindSameBucket = "SAME_BUCKET"
	KindWindow     = "WINDOW"
)

type Options struct {
	Window                time.Duration
	FrequencyToleranceKHz float64
	MaxMatchesPerQSO      int
	DenseMatchIDs         bool
	LoadedAt              time.Time
	SpotterProfiles       map[string]SpotterProfile
}

type SpotterProfile struct {
	SpotterWeight float64
	BaselineDB    float64
	Quality       string
}

type Match struct {
	MatchID               uint64
	ContestID             string
	LogNumID              uint64
	LogID                 string
	QSOID                 uint64
	SpotID                uint64
	QSOActivity5MID       uint64
	QSOActivity5MKey      string
	SpotActivity5MID      uint64
	SpotActivity5MKey     string
	StationCall           string
	StationPrefix         string
	StationContinent      string
	WorkedCall            string
	WorkedPrefix          string
	WorkedContinent       string
	SpotterCall           string
	SpotterPrefix         string
	SpotterContinent      string
	DXPrefix              string
	DXContinent           string
	Band                  string
	Mode                  string
	QSOAt                 time.Time
	SpottedAt             time.Time
	QSOFrequencyKHz       float64
	SpotFrequencyKHz      float64
	TimeDeltaSeconds      int
	AbsTimeDeltaSeconds   int
	FrequencyDeltaKHz     float64
	AbsFrequencyDeltaKHz  float64
	SignalDB              int
	SpeedWPM              int
	MatchWindowSeconds    int
	FrequencyToleranceKHz float64
	SameActivityBucket    int
	MatchKind             string
	TimeScore             float64
	FrequencyScore        float64
	MatchScore            float64
	MatchRank             int
	IsBestMatch           int
	SpotterProfileFound   int
	SpotterProfileQuality string
	SpotterWeight         float64
	SpotterBaselineDB     float64
	NormalizedSignalDB    float64
	WeightedSignalDB      float64
	Source                string
	LoadedAt              time.Time
}

func MatchQSOsToSpots(qsos []cabrillo.QSO, spots []rbn.Spot, options Options) []Match {
	window := options.Window
	if window <= 0 {
		window = 5 * time.Minute
	}
	loadedAt := options.LoadedAt.UTC()
	if loadedAt.IsZero() {
		loadedAt = time.Now().UTC()
	}

	spotsByKey := map[string][]rbn.Spot{}
	for _, spot := range spots {
		key := matchKey(spot.DXCall, spot.Band, activityModeForSpot(spot))
		spotsByKey[key] = append(spotsByKey[key], spot)
	}
	for key := range spotsByKey {
		sort.Slice(spotsByKey[key], func(i, j int) bool {
			if spotsByKey[key][i].SpottedAt.Equal(spotsByKey[key][j].SpottedAt) {
				return spotsByKey[key][i].SpotID < spotsByKey[key][j].SpotID
			}
			return spotsByKey[key][i].SpottedAt.Before(spotsByKey[key][j].SpottedAt)
		})
	}
	spotterProfiles := normalizeSpotterProfiles(options.SpotterProfiles)

	var matches []Match
	for _, qso := range qsos {
		candidates := matchingCandidates(qso, spotsByKey[matchKey(qso.StationCall, qso.Band, qso.Mode)], window, options.FrequencyToleranceKHz)
		if options.MaxMatchesPerQSO > 0 && len(candidates) > options.MaxMatchesPerQSO {
			candidates = candidates[:options.MaxMatchesPerQSO]
		}
		for i, spot := range candidates {
			profile, profileFound := lookupSpotterProfile(spotterProfiles, spot.SpotterCall)
			match := NewMatchWithProfile(qso, spot, window, options.FrequencyToleranceKHz, loadedAt, profile, profileFound)
			match.MatchRank = i + 1
			if i == 0 {
				match.IsBestMatch = 1
			}
			matches = append(matches, match)
		}
	}
	if options.DenseMatchIDs {
		for i := range matches {
			matches[i].MatchID = uint64(i + 1)
		}
	}
	return matches
}

func NewMatch(qso cabrillo.QSO, spot rbn.Spot, window time.Duration, frequencyToleranceKHz float64, loadedAt time.Time) Match {
	return NewMatchWithProfile(qso, spot, window, frequencyToleranceKHz, loadedAt, SpotterProfile{}, false)
}

func NewMatchWithProfile(qso cabrillo.QSO, spot rbn.Spot, window time.Duration, frequencyToleranceKHz float64, loadedAt time.Time, profile SpotterProfile, profileFound bool) Match {
	timeDelta := int(spot.SpottedAt.Sub(qso.QSOAt).Seconds())
	frequencyDelta := spot.FrequencyKHz - qso.FrequencyKHz
	sameBucket := 0
	kind := KindWindow
	if qso.Activity5MID == spot.Activity5MID {
		sameBucket = 1
		kind = KindSameBucket
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	timeScore, frequencyScore, matchScore := scoreMatch(absInt(timeDelta), window, math.Abs(frequencyDelta), frequencyToleranceKHz, spot.SignalDB)
	loadedAt = loadedAt.UTC()
	if loadedAt.IsZero() {
		loadedAt = time.Now().UTC()
	}
	spotterWeight := profile.SpotterWeight
	if spotterWeight <= 0 {
		spotterWeight = 1
	}
	spotterBaselineDB := profile.BaselineDB
	spotterProfileQuality := strings.TrimSpace(profile.Quality)
	if spotterProfileQuality == "" {
		spotterProfileQuality = "unknown"
	}
	spotterProfileFound := 0
	if profileFound {
		spotterProfileFound = 1
	}
	normalizedSignalDB := float64(spot.SignalDB) - spotterBaselineDB
	weightedSignalDB := normalizedSignalDB * spotterWeight
	return Match{
		MatchID:               StableMatchID(qso.QSOID, spot.SpotID),
		ContestID:             qso.ContestID,
		LogNumID:              qso.LogNumID,
		LogID:                 qso.LogID,
		QSOID:                 qso.QSOID,
		SpotID:                spot.SpotID,
		QSOActivity5MID:       qso.Activity5MID,
		QSOActivity5MKey:      qso.Activity5MKey,
		SpotActivity5MID:      spot.Activity5MID,
		SpotActivity5MKey:     spot.Activity5MKey,
		StationCall:           qso.StationCall,
		StationPrefix:         qso.StationPrefix,
		StationContinent:      qso.StationContinent,
		WorkedCall:            qso.WorkedCall,
		WorkedPrefix:          qso.WorkedPrefix,
		WorkedContinent:       qso.WorkedContinent,
		SpotterCall:           spot.SpotterCall,
		SpotterPrefix:         spot.SpotterPrefix,
		SpotterContinent:      spot.SpotterContinent,
		DXPrefix:              spot.DXPrefix,
		DXContinent:           spot.DXContinent,
		Band:                  qso.Band,
		Mode:                  qso.Mode,
		QSOAt:                 qso.QSOAt,
		SpottedAt:             spot.SpottedAt,
		QSOFrequencyKHz:       qso.FrequencyKHz,
		SpotFrequencyKHz:      spot.FrequencyKHz,
		TimeDeltaSeconds:      timeDelta,
		AbsTimeDeltaSeconds:   absInt(timeDelta),
		FrequencyDeltaKHz:     frequencyDelta,
		AbsFrequencyDeltaKHz:  math.Abs(frequencyDelta),
		SignalDB:              spot.SignalDB,
		SpeedWPM:              spot.SpeedWPM,
		MatchWindowSeconds:    int(window.Seconds()),
		FrequencyToleranceKHz: frequencyToleranceKHz,
		SameActivityBucket:    sameBucket,
		MatchKind:             kind,
		TimeScore:             timeScore,
		FrequencyScore:        frequencyScore,
		MatchScore:            matchScore,
		SpotterProfileFound:   spotterProfileFound,
		SpotterProfileQuality: spotterProfileQuality,
		SpotterWeight:         spotterWeight,
		SpotterBaselineDB:     spotterBaselineDB,
		NormalizedSignalDB:    normalizedSignalDB,
		WeightedSignalDB:      weightedSignalDB,
		Source:                "materialized",
		LoadedAt:              loadedAt,
	}
}

func normalizeSpotterProfiles(input map[string]SpotterProfile) map[string]SpotterProfile {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]SpotterProfile, len(input))
	for call, profile := range input {
		normalizedCall, ok := rbn.NormalizeCallsign(call)
		if !ok {
			continue
		}
		output[normalizedCall] = profile
	}
	return output
}

func lookupSpotterProfile(profiles map[string]SpotterProfile, spotterCall string) (SpotterProfile, bool) {
	if len(profiles) == 0 {
		return SpotterProfile{}, false
	}
	call, ok := rbn.NormalizeCallsign(spotterCall)
	if !ok {
		return SpotterProfile{}, false
	}
	profile, ok := profiles[call]
	return profile, ok
}

func StableMatchID(qsoID uint64, spotID uint64) uint64 {
	h := fnv.New64a()
	writePart := func(v string) {
		_, _ = h.Write([]byte(v))
		_, _ = h.Write([]byte{0})
	}
	writePart(strconv.FormatUint(qsoID, 10))
	writePart(strconv.FormatUint(spotID, 10))
	id := h.Sum64() & ((uint64(1) << 63) - 1)
	if id == 0 {
		return 1
	}
	return id
}

func matchingCandidates(qso cabrillo.QSO, spots []rbn.Spot, window time.Duration, frequencyToleranceKHz float64) []rbn.Spot {
	if len(spots) == 0 {
		return nil
	}
	lower := qso.QSOAt.Add(-window)
	upper := qso.QSOAt.Add(window)
	start := sort.Search(len(spots), func(i int) bool {
		return !spots[i].SpottedAt.Before(lower)
	})
	candidates := make([]rbn.Spot, 0)
	for i := start; i < len(spots); i++ {
		spot := spots[i]
		if spot.SpottedAt.After(upper) {
			break
		}
		if frequencyToleranceKHz > 0 && math.Abs(spot.FrequencyKHz-qso.FrequencyKHz) > frequencyToleranceKHz {
			continue
		}
		candidates = append(candidates, spot)
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftDelta := absDuration(candidates[i].SpottedAt.Sub(qso.QSOAt))
		rightDelta := absDuration(candidates[j].SpottedAt.Sub(qso.QSOAt))
		if leftDelta != rightDelta {
			return leftDelta < rightDelta
		}
		leftFreq := math.Abs(candidates[i].FrequencyKHz - qso.FrequencyKHz)
		rightFreq := math.Abs(candidates[j].FrequencyKHz - qso.FrequencyKHz)
		if leftFreq != rightFreq {
			return leftFreq < rightFreq
		}
		if candidates[i].SignalDB != candidates[j].SignalDB {
			return candidates[i].SignalDB > candidates[j].SignalDB
		}
		return candidates[i].SpotID < candidates[j].SpotID
	})
	return candidates
}

func matchKey(call string, band string, mode string) string {
	return strings.Join([]string{
		strings.ToUpper(strings.TrimSpace(call)),
		strings.ToUpper(strings.TrimSpace(band)),
		strings.ToUpper(strings.TrimSpace(mode)),
	}, "|")
}

func activityModeForSpot(spot rbn.Spot) string {
	mode := strings.TrimSpace(spot.TransmitMode)
	if mode == "" {
		mode = spot.Mode
	}
	return mode
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func scoreMatch(absTimeDeltaSeconds int, window time.Duration, absFrequencyDeltaKHz float64, frequencyToleranceKHz float64, signalDB int) (float64, float64, float64) {
	windowSeconds := window.Seconds()
	if windowSeconds <= 0 {
		windowSeconds = (5 * time.Minute).Seconds()
	}
	timeScore := 100 * (1 - clamp01(float64(absTimeDeltaSeconds)/windowSeconds))

	var frequencyScore float64
	if frequencyToleranceKHz > 0 {
		frequencyScore = 100 * (1 - clamp01(absFrequencyDeltaKHz/frequencyToleranceKHz))
	} else {
		frequencyScore = 100 / (1 + math.Max(absFrequencyDeltaKHz, 0))
	}

	signalScore := clamp(float64(signalDB)*2, 0, 100)
	matchScore := (timeScore * 0.80) + (frequencyScore * 0.15) + (signalScore * 0.05)
	return roundScore(timeScore), roundScore(frequencyScore), roundScore(matchScore)
}

func clamp01(v float64) float64 {
	return clamp(v, 0, 1)
}

func clamp(v float64, min float64, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func roundScore(v float64) float64 {
	return math.Round(v*1000) / 1000
}
