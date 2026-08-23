package rbn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type LoaderClient struct {
	Target string
	Client *http.Client
}

type LoaderResponse struct {
	Accepted int `json:"accepted"`
	Failed   int `json:"failed"`
}

func (c LoaderClient) PostEvents(ctx context.Context, events []interface{}) (LoaderResponse, error) {
	if len(events) == 0 {
		return LoaderResponse{}, nil
	}
	body, err := json.Marshal(map[string]interface{}{"events": events})
	if err != nil {
		return LoaderResponse{}, err
	}

	target := strings.TrimSpace(c.Target)
	if target == "" {
		target = "http://127.0.0.1:8088/ingest/json"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return LoaderResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return LoaderResponse{}, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LoaderResponse{}, fmt.Errorf("loader POST %s: %s: %s", target, resp.Status, strings.TrimSpace(string(respBody)))
	}
	var loaderResp LoaderResponse
	if len(bytes.TrimSpace(respBody)) == 0 {
		return loaderResp, nil
	}
	if err := json.Unmarshal(respBody, &loaderResp); err != nil {
		return LoaderResponse{}, fmt.Errorf("decode loader response: %w", err)
	}
	return loaderResp, nil
}
