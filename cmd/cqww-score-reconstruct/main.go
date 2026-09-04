package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/cqwwscore"
)

type output struct {
	Source       string            `json:"source"`
	ClaimedScore int64             `json:"claimed_score"`
	MatchesClaim bool              `json:"matches_claim"`
	Summary      cqwwscore.Summary `json:"summary"`
	States       []cqwwscore.State `json:"states,omitempty"`
}

func main() {
	log.SetFlags(0)
	ctyPath := flag.String("cty-dat", "data/cty/cty.dat", "CTY/DXCC data file")
	includeStates := flag.Bool("states", false, "include cumulative state after every QSO")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP source timeout")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: cqww-score-reconstruct [flags] <Cabrillo path or URL> [...]")
		flag.PrintDefaults()
		os.Exit(2)
	}
	db, err := callsign.LoadFile(*ctyPath)
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	for _, source := range flag.Args() {
		r, err := openSource(context.Background(), source, *timeout)
		if err != nil {
			log.Fatalf("open %s: %v", source, err)
		}
		contestLog, qsos, _, parseErr := cabrillo.Parse(r, cabrillo.ParseOptions{SourceFile: cabrillo.SourceLabel(source), CallsignDB: db})
		closeErr := r.Close()
		if parseErr != nil {
			log.Fatalf("parse %s: %v", source, parseErr)
		}
		if closeErr != nil {
			log.Fatalf("close %s: %v", source, closeErr)
		}
		states, summary, err := cqwwscore.Reconstruct(contestLog, qsos, db)
		if err != nil {
			log.Fatalf("score %s: %v", source, err)
		}
		result := output{Source: source, ClaimedScore: contestLog.ClaimedScore, MatchesClaim: summary.Score == contestLog.ClaimedScore, Summary: summary}
		if *includeStates {
			result.States = states
		}
		if err := encoder.Encode(result); err != nil {
			log.Fatal(err)
		}
	}
}

func openSource(ctx context.Context, source string, timeout time.Duration) (io.ReadCloser, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "radiosport-data-lab/0.1")
		resp, err := (&http.Client{Timeout: timeout}).Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("GET %s: %s", source, resp.Status)
		}
		return resp.Body, nil
	}
	return os.Open(source)
}
