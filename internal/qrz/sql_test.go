package qrz

import (
	"testing"
	"time"
)

func TestProfileSQLArgsMatchInsertShape(t *testing.T) {
	lat := 37.1
	lon := -121.9
	profile := Profile{
		Callsign:         "N7ZG",
		DXCCID:           291,
		DXCCPrefix:       "K",
		Continent:        "NA",
		CountryName:      "United States",
		QRZCCode:         840,
		FirstName:        "Guy",
		LastName:         "Molinari",
		Latitude:         &lat,
		Longitude:        &lon,
		LicenseIssueDate: "2020-01-02",
		LicenseExpDate:   "2030-01-02",
		LookupStatus:     "found",
		LookupTime:       time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC),
	}
	args := ProfileSQLArgs(profile)
	if got, want := len(args), 19; got != want {
		t.Fatalf("len(args)=%d want %d", got, want)
	}
	if args[15] != "2020-01-02 00:00:00" || args[16] != "2030-01-02 00:00:00" {
		t.Fatalf("unexpected dates: %#v %#v", args[15], args[16])
	}
	if args[18] != "2026-08-22 11:00:00" {
		t.Fatalf("unexpected lookup time: %#v", args[18])
	}
}

func TestProfileUpdateSQLArgsMoveCallsignToWhereClause(t *testing.T) {
	profile := Profile{
		Callsign:     "N7ZG",
		DXCCID:       291,
		DXCCPrefix:   "K",
		Continent:    "NA",
		CountryName:  "United States",
		LookupStatus: "found",
		LookupTime:   time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC),
	}
	args := ProfileUpdateSQLArgs(profile)
	if got, want := len(args), 19; got != want {
		t.Fatalf("len(args)=%d want %d", got, want)
	}
	if args[0] != 291 {
		t.Fatalf("first arg=%#v, want dxcc id", args[0])
	}
	if args[len(args)-1] != "N7ZG" {
		t.Fatalf("last arg=%#v, want callsign", args[len(args)-1])
	}
}

func TestNotFoundProfileUsesQueryableDefaults(t *testing.T) {
	profile := NotFoundProfile(" nope ", time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC))
	args := ProfileSQLArgs(profile)
	if profile.Callsign != "NOPE" || profile.LookupStatus != "not_found" {
		t.Fatalf("unexpected not-found profile: %#v", profile)
	}
	if args[2] != unknownString || args[3] != unknownString || args[4] != unknownString {
		t.Fatalf("expected unknown defaults: %#v", args)
	}
	if args[15] != "1970-01-01 00:00:00" || args[16] != "1970-01-01 00:00:00" {
		t.Fatalf("unexpected missing dates: %#v %#v", args[15], args[16])
	}
}

func TestPendingProfileUsesQueryableDefaults(t *testing.T) {
	profile := PendingProfile(" n7zg ", time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC))
	args := ProfileSQLArgs(profile)
	if profile.Callsign != "N7ZG" || profile.LookupStatus != PendingLookupStatus {
		t.Fatalf("unexpected pending profile: %#v", profile)
	}
	if args[2] != unknownString || args[3] != unknownString || args[4] != unknownString {
		t.Fatalf("expected unknown defaults: %#v", args)
	}
	if args[18] != "2026-08-22 11:00:00" {
		t.Fatalf("unexpected lookup time: %#v", args[18])
	}
}
