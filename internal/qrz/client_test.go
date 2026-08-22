package qrz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLookupLogsInAndMapsProfile(t *testing.T) {
	var loginCalls, lookupCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch {
		case query.Get("username") == "u" && query.Get("password") == "p":
			loginCalls++
			_, _ = w.Write([]byte(`<QRZDatabase><Session><Key>abc</Key></Session></QRZDatabase>`))
		case query.Get("s") == "abc" && query.Get("callsign") == "N7ZG":
			lookupCalls++
			_, _ = w.Write([]byte(`<QRZDatabase><Callsign>
				<call>N7ZG</call>
				<dxcc>291</dxcc>
				<fname>Guy</fname>
				<name>Molinari</name>
				<state>CA</state>
				<county>Santa Clara</county>
				<grid>CM97</grid>
				<lat>37.1</lat>
				<lon>-121.9</lon>
				<ccode>840</ccode>
				<country>United States</country>
				<cqzone>3</cqzone>
				<ituzone>6</ituzone>
				<efdate>2020-01-02</efdate>
				<expdate>2030-01-02</expdate>
			</Callsign></QRZDatabase>`))
		default:
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	now := time.Date(2026, 8, 22, 10, 30, 0, 0, time.UTC)
	client := NewClient("u", "p", WithBaseURL(server.URL), WithClock(func() time.Time { return now }))
	profile, err := client.Lookup(context.Background(), " n7zg ")
	if err != nil {
		t.Fatal(err)
	}
	if loginCalls != 1 || lookupCalls != 1 {
		t.Fatalf("loginCalls=%d lookupCalls=%d", loginCalls, lookupCalls)
	}
	if profile.Callsign != "N7ZG" || profile.FirstName != "Guy" || profile.LastName != "Molinari" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	if profile.DXCCID != 291 || profile.QRZCCode != 840 || profile.CQZone != 3 || profile.ITUZone != 6 {
		t.Fatalf("unexpected numeric fields: %#v", profile)
	}
	if profile.Latitude == nil || *profile.Latitude != 37.1 || profile.Longitude == nil || *profile.Longitude != -121.9 {
		t.Fatalf("unexpected coordinates: %#v %#v", profile.Latitude, profile.Longitude)
	}
	if profile.LicenseIssueDate != "2020-01-02" || profile.LicenseExpDate != "2030-01-02" {
		t.Fatalf("unexpected dates: %#v", profile)
	}
	if !profile.LookupTime.Equal(now) || profile.LookupStatus != "found" {
		t.Fatalf("unexpected lookup state: %#v", profile)
	}
}

func TestLookupRetriesAfterSessionTimeout(t *testing.T) {
	var loginCalls, lookupCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Has("username") {
			loginCalls++
			_, _ = w.Write([]byte(`<QRZDatabase><Session><Key>abc</Key></Session></QRZDatabase>`))
			return
		}
		lookupCalls++
		if lookupCalls == 1 {
			_, _ = w.Write([]byte(`<QRZDatabase><Session><Error>Session Timeout</Error></Session></QRZDatabase>`))
			return
		}
		_, _ = w.Write([]byte(`<QRZDatabase><Callsign><call>K1ABC</call><country>United States</country></Callsign></QRZDatabase>`))
	}))
	defer server.Close()

	client := NewClient("u", "p", WithBaseURL(server.URL))
	if _, err := client.Lookup(context.Background(), "K1ABC"); err != nil {
		t.Fatal(err)
	}
	if loginCalls != 2 || lookupCalls != 2 {
		t.Fatalf("loginCalls=%d lookupCalls=%d", loginCalls, lookupCalls)
	}
}

func TestLookupCachesNotFound(t *testing.T) {
	var lookupCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("username") {
			_, _ = w.Write([]byte(`<QRZDatabase><Session><Key>abc</Key></Session></QRZDatabase>`))
			return
		}
		lookupCalls++
		_, _ = w.Write([]byte(`<QRZDatabase><Session><Error>Not found: BADCALL</Error></Session></QRZDatabase>`))
	}))
	defer server.Close()

	client := NewClient("u", "p", WithBaseURL(server.URL))
	if _, err := client.Lookup(context.Background(), "BADCALL"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("first lookup err=%v", err)
	}
	if _, err := client.Lookup(context.Background(), "BADCALL"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cached lookup err=%v", err)
	}
	if lookupCalls != 1 {
		t.Fatalf("lookupCalls=%d", lookupCalls)
	}
}

func TestNewClientFromEnvRequiresCredentials(t *testing.T) {
	t.Setenv("QRZ_USERNAME", "")
	t.Setenv("QRZ_PASSWORD", "")
	if _, err := NewClientFromEnv(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("err=%v", err)
	}
}
