package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLoaderEndpointTargetsLoaderControlPath(t *testing.T) {
	got, err := loaderEndpoint("http://127.0.0.1:8088/ingest/json?ignored=true", "/commit")
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:8088/commit"
	if got != want {
		t.Fatalf("loaderEndpoint() = %q, want %q", got, want)
	}
}

func TestFetchLoaderIdleReadsStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/stats" {
			t.Fatalf("path = %q, want /stats", r.URL.Path)
		}
		writeJSON(t, w, loaderStatsResponse{
			Router: loaderStatsRouterResponse{
				TotalQueued:      0,
				OpenSessionCount: 0,
			},
		})
	}))
	defer server.Close()

	idle, err := fetchLoaderIdle(context.Background(), server.URL+"/stats", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !idle {
		t.Fatal("fetchLoaderIdle() = false, want true")
	}
}

func TestFetchLoaderIdleSeesQueuedWork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, loaderStatsResponse{
			Router: loaderStatsRouterResponse{
				TotalQueued:      1,
				OpenSessionCount: 0,
			},
		})
	}))
	defer server.Close()

	idle, err := fetchLoaderIdle(context.Background(), server.URL+"/stats", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if idle {
		t.Fatal("fetchLoaderIdle() = true, want false")
	}
}

func TestCommitLoaderPostsCommit(t *testing.T) {
	var committed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/commit" {
			t.Fatalf("path = %q, want /commit", r.URL.Path)
		}
		committed = true
		writeJSON(t, w, loaderCommitTestResponse{
			Status: "ok",
			Commit: loaderCommitCountResponse{
				CommitCount: 3,
			},
		})
	}))
	defer server.Close()

	commits, err := commitLoader(context.Background(), loadConfig{
		target:        server.URL + "/ingest/json",
		commitTimeout: time.Second,
		timeout:       time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !committed {
		t.Fatal("commit endpoint was not called")
	}
	if commits != 3 {
		t.Fatalf("commitLoader() = %d, want 3", commits)
	}
}

type loaderStatsResponse struct {
	Router loaderStatsRouterResponse `json:"router"`
}

type loaderStatsRouterResponse struct {
	TotalQueued      int `json:"total_queued"`
	OpenSessionCount int `json:"open_session_count"`
}

type loaderCommitTestResponse struct {
	Status string                    `json:"status"`
	Commit loaderCommitCountResponse `json:"commit"`
}

type loaderCommitCountResponse struct {
	CommitCount int `json:"commit_count"`
}

func writeJSON(t *testing.T, w http.ResponseWriter, value interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatal(err)
	}
}
