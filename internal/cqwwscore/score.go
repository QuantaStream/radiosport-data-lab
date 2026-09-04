package cqwwscore

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
)

type State struct {
	At                 time.Time
	StationCall        string
	Band               string
	WorkedCall         string
	QSOCount           int
	CountedQSOCount    int
	DuplicateCount     int
	UnresolvedCount    int
	QSOPoints          int
	CountryMultipliers int
	ZoneMultipliers    int
	MultiplierCount    int
	Score              int64
	QSOAdded           int
	PointsAdded        int
	CountryAdded       int
	ZoneAdded          int
	Duplicate          bool
	Unresolved         bool
}

type Summary struct {
	StationCall        string
	SubmittedQSOs      int
	CountedQSOs        int
	Duplicates         int
	Unresolved         int
	QSOPoints          int
	CountryMultipliers int
	ZoneMultipliers    int
	Multipliers        int
	Score              int64
}

// Reconstruct calculates the raw score represented by a submitted CQ WW log.
// It intentionally does not attempt adjudication.
func Reconstruct(log cabrillo.Log, qsos []cabrillo.QSO, db *callsign.Database) ([]State, Summary, error) {
	if db == nil {
		return nil, Summary{}, fmt.Errorf("callsign database is required")
	}
	station, err := db.Parse(log.StationCall)
	if err != nil || !station.Valid {
		return nil, Summary{}, fmt.Errorf("resolve station call %s: %w", log.StationCall, err)
	}
	ordered := append([]cabrillo.QSO(nil), qsos...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].QSOAt.Before(ordered[j].QSOAt) })

	dupes := map[string]struct{}{}
	countries := map[string]struct{}{}
	zones := map[string]struct{}{}
	states := make([]State, 0, len(ordered))
	summary := Summary{StationCall: log.StationCall, SubmittedQSOs: len(ordered)}

	for _, qso := range ordered {
		state := State{At: qso.QSOAt, StationCall: log.StationCall, Band: qso.Band, WorkedCall: qso.WorkedCall}
		dupeKey := strings.ToUpper(qso.Band + "|" + qso.WorkedCall)
		if _, exists := dupes[dupeKey]; exists {
			summary.Duplicates++
			state.Duplicate = true
		} else {
			dupes[dupeKey] = struct{}{}
			worked, resolveErr := db.Parse(qso.WorkedCall)
			zone, zoneErr := strconv.Atoi(strings.TrimSpace(qso.ReceivedExchange))
			if resolveErr != nil || !worked.Valid || worked.Aeronautical || zoneErr != nil || zone < 1 || zone > 40 {
				summary.Unresolved++
				state.Unresolved = true
			} else {
				points := 0
				if !worked.Maritime {
					points = qsoPoints(station, worked)
				}
				summary.CountedQSOs++
				summary.QSOPoints += points
				state.QSOAdded = 1
				state.PointsAdded = points

				if !worked.Maritime {
					countryKey := strings.ToUpper(qso.Band + "|" + worked.PrimaryPrefix)
					if _, exists := countries[countryKey]; !exists {
						countries[countryKey] = struct{}{}
						summary.CountryMultipliers++
						state.CountryAdded = 1
					}
				}
				zoneKey := strings.ToUpper(qso.Band + "|" + strconv.Itoa(zone))
				if _, exists := zones[zoneKey]; !exists {
					zones[zoneKey] = struct{}{}
					summary.ZoneMultipliers++
					state.ZoneAdded = 1
				}
			}
		}
		summary.Multipliers = summary.CountryMultipliers + summary.ZoneMultipliers
		summary.Score = int64(summary.QSOPoints) * int64(summary.Multipliers)
		state.QSOCount = len(states) + 1
		state.CountedQSOCount = summary.CountedQSOs
		state.DuplicateCount = summary.Duplicates
		state.UnresolvedCount = summary.Unresolved
		state.QSOPoints = summary.QSOPoints
		state.CountryMultipliers = summary.CountryMultipliers
		state.ZoneMultipliers = summary.ZoneMultipliers
		state.MultiplierCount = summary.Multipliers
		state.Score = summary.Score
		states = append(states, state)
	}
	return states, summary, nil
}

func qsoPoints(station, worked callsign.Station) int {
	if strings.EqualFold(station.PrimaryPrefix, worked.PrimaryPrefix) {
		return 0
	}
	if strings.EqualFold(station.Continent, worked.Continent) {
		if strings.EqualFold(station.Continent, "NA") {
			return 2
		}
		return 1
	}
	return 3
}
