package cqwwscore

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/cabrillo"
	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
)

const testCTY = `Canary Islands:              33:  36:  AF:   28.32:    15.85:     0.0:  EA8:
    EA8;
Spain:                       14:  37:  EU:   40.37:     4.88:    -1.0:  EA:
    EA;
Morocco:                     33:  37:  AF:   32.00:     5.00:     0.0:  CN:
    CN;
United States:               05:  08:  NA:   37.53:    91.67:     5.0:  K:
    K;
Asiatic Russia:              17:  30:  AS:   55.88:   -84.08:    -7.0:  UA9:
    UA9;
`

func TestReconstructCQWWPointsMultipliersAndDuplicates(t *testing.T) {
	db, err := callsign.Load(strings.NewReader(testCTY))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2025, 11, 29, 0, 0, 0, 0, time.UTC)
	log := cabrillo.Log{StationCall: "EA8A"}
	qsos := []cabrillo.QSO{
		{QSOAt: base, StationCall: "EA8A", WorkedCall: "EA1AAA", Band: "20m", ReceivedExchange: "14"},
		{QSOAt: base.Add(time.Minute), StationCall: "EA8A", WorkedCall: "K1AAA", Band: "20m", ReceivedExchange: "05"},
		{QSOAt: base.Add(2 * time.Minute), StationCall: "EA8A", WorkedCall: "CN2AA", Band: "20m", ReceivedExchange: "33"},
		{QSOAt: base.Add(3 * time.Minute), StationCall: "EA8A", WorkedCall: "K1AAA", Band: "20m", ReceivedExchange: "05"},
		{QSOAt: base.Add(4 * time.Minute), StationCall: "EA8A", WorkedCall: "K1AAA", Band: "15m", ReceivedExchange: "05"},
		{QSOAt: base.Add(5 * time.Minute), StationCall: "EA8A", WorkedCall: "UA9AAA/MM", Band: "20m", ReceivedExchange: "34"},
	}
	states, got, err := Reconstruct(log, qsos, db)
	if err != nil {
		t.Fatal(err)
	}
	if got.SubmittedQSOs != 6 || got.CountedQSOs != 5 || got.Duplicates != 1 || got.Unresolved != 0 {
		t.Fatalf("counts = %+v", got)
	}
	// EA=3 points, K=3, CN=1, K on another band=3.
	if got.QSOPoints != 10 || got.CountryMultipliers != 4 || got.ZoneMultipliers != 5 || got.Score != 90 {
		t.Fatalf("score = %+v", got)
	}
	if !states[3].Duplicate || states[3].Score != states[2].Score {
		t.Fatalf("duplicate state = %+v", states[3])
	}
}
