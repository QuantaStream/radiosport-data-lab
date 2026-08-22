package rbn

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParseTelnetSpot parses a classic RBN telnet line such as:
//
//	DX de G4IRN-#: 14054.4 KC2SIZ CW 25 dB 13 WPM CQ 0000Z
//
// Prefix and continent fields are filled when a lookup implementation is provided.
func ParseTelnetSpot(line string, now time.Time, lookup CallsignLookup) (Spot, bool, error) {
	fields := strings.Fields(line)
	if len(fields) < 12 || fields[0] != "DX" || fields[1] != "de" {
		return Spot{}, false, nil
	}

	spotterCall, ok := NormalizeCallsign(cleanTelnetSpotter(fields[2]))
	if !ok {
		return Spot{}, false, fmt.Errorf("invalid telnet spotter callsign %q", fields[2])
	}
	dxCall, ok := NormalizeCallsign(fields[4])
	if !ok {
		return Spot{}, false, fmt.Errorf("invalid telnet dx callsign %q", fields[4])
	}

	freqKHz, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return Spot{}, false, fmt.Errorf("parse telnet frequency %q: %w", fields[3], err)
	}
	band, ok := BandForFrequencyKHz(freqKHz)
	if !ok {
		return Spot{}, false, fmt.Errorf("cannot map frequency %.3f kHz to amateur band", freqKHz)
	}
	signalDB, err := strconv.Atoi(fields[6])
	if err != nil {
		return Spot{}, false, fmt.Errorf("parse telnet db %q: %w", fields[6], err)
	}
	speedWPM, err := strconv.Atoi(fields[8])
	if err != nil {
		return Spot{}, false, fmt.Errorf("parse telnet speed %q: %w", fields[8], err)
	}

	timeFieldIndex := 11
	if fields[timeFieldIndex] == "B" && len(fields) > 12 {
		timeFieldIndex = 12
	}
	spottedAt, err := parseTelnetUTCMinute(fields[timeFieldIndex], now.UTC())
	if err != nil {
		return Spot{}, false, err
	}

	spot := Spot{
		SpottedAt:    spottedAt,
		SpotterCall:  spotterCall,
		DXCall:       dxCall,
		FrequencyHz:  int64(math.Round(freqKHz * 1000)),
		Band:         band,
		Mode:         fields[10],
		SignalDB:     signalDB,
		SpeedWPM:     speedWPM,
		TransmitMode: fields[5],
		Source:       "telnet",
	}
	if lookup != nil {
		if info, ok := lookup.LookupCallsign(spot.SpotterCall); ok {
			spot.SpotterPrefix = info.Prefix
			spot.SpotterContinent = info.Continent
		}
		if info, ok := lookup.LookupCallsign(spot.DXCall); ok {
			spot.DXPrefix = info.Prefix
			spot.DXContinent = info.Continent
		}
	}
	if spot.SpotterPrefix == "" {
		spot.SpotterPrefix = UnknownValue
	}
	if spot.SpotterContinent == "" {
		spot.SpotterContinent = UnknownValue
	}
	if spot.DXPrefix == "" {
		spot.DXPrefix = UnknownValue
	}
	if spot.DXContinent == "" {
		spot.DXContinent = UnknownValue
	}
	spot.SpotID = StableSpotID(spot)
	return spot, true, nil
}

func parseTelnetUTCMinute(input string, now time.Time) (time.Time, error) {
	value := strings.TrimSpace(input)
	if len(value) != 5 || !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("invalid telnet UTC minute %q", input)
	}
	hour, err := strconv.Atoi(value[0:2])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse telnet hour %q: %w", value[0:2], err)
	}
	minute, err := strconv.Atoi(value[2:4])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse telnet minute %q: %w", value[2:4], err)
	}
	spottedAt := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if now.Hour() == 0 && hour == 23 {
		spottedAt = spottedAt.AddDate(0, 0, -1)
	}
	if now.Hour() == 23 && hour == 0 {
		spottedAt = spottedAt.AddDate(0, 0, 1)
	}
	return spottedAt, nil
}

func cleanTelnetSpotter(input string) string {
	value := strings.TrimSuffix(strings.TrimSpace(input), ":")
	value = strings.TrimSuffix(value, "-#")
	return value
}
