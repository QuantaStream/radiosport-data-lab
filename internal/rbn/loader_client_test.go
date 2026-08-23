package rbn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoaderClientPostsEventsBatch(t *testing.T) {
	var seen int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Method, http.MethodPost; got != want {
			t.Fatalf("method = %s, want %s", got, want)
		}
		if got, want := r.URL.Path, "/ingest/json"; got != want {
			t.Fatalf("path = %s, want %s", got, want)
		}
		var payload struct {
			Events []map[string]interface{} `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		seen = len(payload.Events)
		_ = json.NewEncoder(w).Encode(LoaderResponse{Accepted: seen})
	}))
	defer server.Close()

	client := LoaderClient{Target: server.URL + "/ingest/json", Client: server.Client()}
	resp, err := client.PostEvents(context.Background(), []interface{}{
		NewSpotEvent(Spot{
			SpotID:           1,
			SpottedAt:        time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC),
			SpotterCall:      "G4IRN",
			SpotterPrefix:    "G",
			SpotterContinent: "EU",
			DXCall:           "KC2SIZ",
			DXPrefix:         "K",
			DXContinent:      "NA",
			FrequencyKHz:     14054.4,
			Band:             "20m",
			Mode:             "CQ",
			SignalDB:         25,
			SpeedWPM:         13,
			TransmitMode:     "CW",
			Source:           SourceArchive,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 1 || resp.Accepted != 1 || resp.Failed != 0 {
		t.Fatalf("seen=%d response=%+v", seen, resp)
	}
}
