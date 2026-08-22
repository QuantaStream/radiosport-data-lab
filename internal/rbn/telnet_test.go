package rbn

import (
	"testing"
	"time"
)

type fakeLookup map[string]CallsignInfo

func (f fakeLookup) LookupCallsign(call string) (CallsignInfo, bool) {
	info, ok := f[call]
	return info, ok
}

func TestParseTelnetSpot(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 30, 0, 0, time.UTC)
	line := "DX de G4IRN-#: 14054.4 KC2SIZ CW 25 dB 13 WPM CQ 0000Z"
	spot, ok, err := ParseTelnetSpot(line, now, fakeLookup{
		"G4IRN":  {Prefix: "G", Continent: "EU"},
		"KC2SIZ": {Prefix: "K", Continent: "NA"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("spot was not recognized")
	}
	if got, want := spot.SpotterCall, "G4IRN"; got != want {
		t.Fatalf("spotter = %q, want %q", got, want)
	}
	if got, want := spot.DXPrefix, "K"; got != want {
		t.Fatalf("dx prefix = %q, want %q", got, want)
	}
	if got, want := spot.Band, "20m"; got != want {
		t.Fatalf("band = %q, want %q", got, want)
	}
	if got, want := spot.TransmitMode, "CW"; got != want {
		t.Fatalf("transmit mode = %q, want %q", got, want)
	}
}

func TestParseTelnetSpotUsesUnknownEnrichmentWithoutLookup(t *testing.T) {
	now := time.Date(2026, 8, 21, 0, 30, 0, 0, time.UTC)
	line := "DX de G4IRN-#: 14054.4 KC2SIZ CW 25 dB 13 WPM CQ 0000Z"
	spot, ok, err := ParseTelnetSpot(line, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("spot was not recognized")
	}
	if got, want := spot.SpotterPrefix, UnknownValue; got != want {
		t.Fatalf("spotter prefix = %q, want %q", got, want)
	}
	if got, want := spot.DXContinent, UnknownValue; got != want {
		t.Fatalf("dx continent = %q, want %q", got, want)
	}
}
