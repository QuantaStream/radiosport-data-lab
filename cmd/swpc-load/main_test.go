package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestYearlyFilename(t *testing.T) {
	tests := []struct {
		product string
		year    int
		want    string
	}{
		{product: "solar", year: 2025, want: "2025_DSD.txt"},
		{product: "geomag", year: 2025, want: "2025_DGD.txt"},
	}

	for _, test := range tests {
		got, err := yearlyFilename(test.product, test.year)
		if err != nil {
			t.Fatalf("yearlyFilename(%q, %d): %v", test.product, test.year, err)
		}
		if got != test.want {
			t.Fatalf("yearlyFilename(%q, %d) = %q, want %q", test.product, test.year, got, test.want)
		}
	}
}

func TestResolveYearlySourceUsesCachedFile(t *testing.T) {
	cacheDir := t.TempDir()
	wantPath := filepath.Join(cacheDir, "2025_DSD.txt")
	if err := os.WriteFile(wantPath, []byte("cached"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveYearlySource(context.Background(), "solar", 2025, cacheDir, "http://127.0.0.1:1", false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got != wantPath {
		t.Fatalf("path = %q, want %q", got, wantPath)
	}
}

func TestResolveYearlySourceDownloadsMissingFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2025_DGD.txt" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("downloaded"))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	got, err := resolveYearlySource(context.Background(), "geomag", 2025, cacheDir, server.URL, false, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(got, "2025_DGD.txt") {
		t.Fatalf("path = %q, want 2025_DGD.txt suffix", got)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "downloaded" {
		t.Fatalf("downloaded data = %q, want downloaded", string(data))
	}
}

func TestResolveYearlySourceFallsBackToSecondBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/archive/2025/2025_DSD.txt" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte("fallback"))
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	baseURLs := "http://127.0.0.1:1," + server.URL + "/archive/{year}"
	got, err := resolveYearlySource(context.Background(), "solar", 2025, cacheDir, baseURLs, false, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "fallback" {
		t.Fatalf("downloaded data = %q, want fallback", string(data))
	}
}

func TestHistoricalSourceURLsExpandYearTemplates(t *testing.T) {
	got := historicalSourceURLs("https://example.test/root, https://example.test/archive/{year}/", "2025_DSD.txt", 2025)
	want := []string{
		"https://example.test/root/2025_DSD.txt",
		"https://example.test/archive/2025/2025_DSD.txt",
	}
	if len(got) != len(want) {
		t.Fatalf("url count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("url %d = %q, want %q", i, got[i], want[i])
		}
	}
}
