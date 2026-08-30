package swpc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultHistoricalBaseURLs = "https://ftp.swpc.noaa.gov/pub/indices/old_indices,https://solar.physics.montana.edu/takeda/NOAA_reports/archive/{year}"

func ResolveYearlySource(ctx context.Context, product string, year int, cacheDir, baseURL string, refresh bool, timeout time.Duration) (string, error) {
	filename, err := YearlyFilename(product, year)
	if err != nil {
		return "", err
	}
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "", fmt.Errorf("cache directory is required for yearly SWPC source")
	}
	path := filepath.Join(cacheDir, filename)
	if !refresh && FileHasContent(path) {
		return path, nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	urls := HistoricalSourceURLs(baseURL, filename, year)
	if len(urls) == 0 {
		return "", fmt.Errorf("historical base URL is required")
	}
	var attempts []string
	for _, sourceURL := range urls {
		if err := DownloadSource(ctx, sourceURL, path, timeout); err != nil {
			attempts = append(attempts, fmt.Sprintf("%s: %v", sourceURL, err))
			continue
		}
		return path, nil
	}
	return "", fmt.Errorf("download %s: all historical sources failed: %s", filename, strings.Join(attempts, "; "))
}

func YearlyFilename(product string, year int) (string, error) {
	if year <= 0 || year > 9999 {
		return "", fmt.Errorf("invalid historical SWPC year %d", year)
	}
	switch product {
	case "solar":
		return fmt.Sprintf("%04d_DSD.txt", year), nil
	case "geomag":
		return fmt.Sprintf("%04d_DGD.txt", year), nil
	default:
		return "", fmt.Errorf("unsupported SWPC historical product %q", product)
	}
}

func HistoricalSourceURLs(baseURLs, filename string, year int) []string {
	yearText := fmt.Sprintf("%04d", year)
	parts := strings.Split(baseURLs, ",")
	urls := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.ReplaceAll(part, "{year}", yearText)
		urls = append(urls, strings.TrimRight(part, "/")+"/"+filename)
	}
	return urls
}

func FileHasContent(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir() && stat.Size() > 0
}

func DownloadSource(ctx context.Context, sourceURL, dest string, timeout time.Duration) error {
	if strings.TrimSpace(sourceURL) == "" {
		return fmt.Errorf("source URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "radiosport-data-lab/0.1")
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: %s", sourceURL, resp.Status)
	}
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	return os.Rename(tmp, dest)
}
