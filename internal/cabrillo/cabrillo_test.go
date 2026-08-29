package cabrillo

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/geo"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const sampleLog = `START-OF-LOG: 3.0
CONTEST: CQ-WW-CW
CALLSIGN: TI8X
CATEGORY-OPERATOR: SINGLE-OP
CATEGORY-ASSISTED: NON-ASSISTED
CATEGORY-BAND: ALL
CATEGORY-POWER: HIGH
CATEGORY-MODE: CW
CATEGORY-TRANSMITTER: ONE
CLAIMED-SCORE: 4476480
QSO:    7058 CW 2025-11-29 0002 TI8X             599 7     K0DU             599  04
QSO:   21010 CW 2025-11-30 2346 TI8X             599 7     JA3YBK           599  25
END-OF-LOG:
`

const sampleCTY = `Costa Rica:                 07:  11:  NA:   10.00:    84.00:     6.0:  TI:
    TI;
Japan:                      25:  45:  AS:   36.40:  -138.38:    -9.0:  JA:
    JA;
`

func TestParseCabrilloCQWWLog(t *testing.T) {
	loadedAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	log, qsos, stats, err := Parse(strings.NewReader(sampleLog), ParseOptions{
		ScopeRegion: "tier1",
		SourceFile:  "ti8x.log",
		LoadedAt:    loadedAt,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if stats.QSOLines != 2 || stats.ParsedQSOs != 2 || stats.RejectedQSOs != 0 {
		t.Fatalf("stats = %#v, want 2 parsed QSOs", stats)
	}
	if log.LogID != "cq-ww-cw-2025:TI8X" || log.ContestID != "cq-ww-cw-2025" {
		t.Fatalf("log ids = %q/%q", log.LogID, log.ContestID)
	}
	if log.StationCall != "TI8X" || log.CategoryOperator != "SINGLE-OP" || log.ClaimedScore != 4476480 {
		t.Fatalf("log = %#v", log)
	}
	if log.QSOCount != 2 || !log.LoadedAt.Equal(loadedAt) {
		t.Fatalf("log count/loaded = %d/%s", log.QSOCount, log.LoadedAt)
	}
	if len(qsos) != 2 {
		t.Fatalf("len(qsos) = %d", len(qsos))
	}
	if qsos[0].QSOAt.Format(time.RFC3339) != "2025-11-29T00:02:00Z" {
		t.Fatalf("qso time = %s", qsos[0].QSOAt)
	}
	if qsos[0].QSODayKey != 20251129 || qsos[0].QSO3HBucketKey != 2025112900 {
		t.Fatalf("qso keys = %d/%d", qsos[0].QSODayKey, qsos[0].QSO3HBucketKey)
	}
	if got, want := qsos[0].QSO5MBucketKey, 202511290000; got != want {
		t.Fatalf("qso_5m_bucket_key = %d, want %d", got, want)
	}
	if got, want := qsos[0].Activity5MKey, "TI8X|40M|CW|202511290000"; got != want {
		t.Fatalf("activity_5m_key = %q, want %q", got, want)
	}
	if qsos[0].Activity5MID == 0 {
		t.Fatal("activity_5m_id = 0, want stable non-zero id")
	}
	if qsos[0].Band != "40m" || qsos[0].WorkedCall != "K0DU" || qsos[0].ReceivedExchange != "04" {
		t.Fatalf("qso[0] = %#v", qsos[0])
	}
	if qsos[1].Band != "15m" || qsos[1].WorkedCall != "JA3YBK" || qsos[1].ReceivedExchange != "25" {
		t.Fatalf("qso[1] = %#v", qsos[1])
	}
	if qsos[0].QSOID == 0 || qsos[0].QSOID == qsos[1].QSOID {
		t.Fatalf("qso ids = %d/%d", qsos[0].QSOID, qsos[1].QSOID)
	}
}

func TestParseCabrilloAddsStationGeoFromCTY(t *testing.T) {
	db, err := callsign.Load(strings.NewReader(sampleCTY))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	log, qsos, _, err := Parse(strings.NewReader(sampleLog), ParseOptions{
		SourceFile: "ti8x.log",
		CallsignDB: db,
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if log.StationCountry != "Costa Rica" || log.StationPrefix != "TI" || log.StationContinent != "NA" {
		t.Fatalf("station enrichment = %+v", log)
	}
	if log.StationLatitude != 10 || log.StationLongitude != -84 {
		t.Fatalf("station location = %v/%v, want 10/-84", log.StationLatitude, log.StationLongitude)
	}
	if log.StationGeoSource != geo.SourceCTY || log.StationGeoConfidence != geo.ConfidenceCountryCentroid {
		t.Fatalf("station geo provenance = %s/%s", log.StationGeoSource, log.StationGeoConfidence)
	}
	if qsos[1].WorkedPrefix != "JA" || qsos[1].WorkedContinent != "AS" {
		t.Fatalf("worked call enrichment = %+v", qsos[1])
	}
	event := NewLogEvent(log)
	if event.Data.StationLatitude != 10 || event.Data.StationLongitude != -84 {
		t.Fatalf("log event geo = %+v", event.Data)
	}
}

func TestNewEventsKeepsParentBeforeQSOs(t *testing.T) {
	log, qsos, _, err := Parse(strings.NewReader(sampleLog), ParseOptions{SourceFile: "ti8x.log"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	events := NewEvents(log, qsos)
	if len(events) != 5 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if event, ok := events[0].(LogEvent); !ok || event.Type != LogEventType || event.Data.LogID != log.LogID {
		t.Fatalf("event[0] = %#v, want parent log event", events[0])
	}
	if event, ok := events[1].(rbn.Activity5MBucketEvent); !ok || event.Type != rbn.Activity5MBucketEventType {
		t.Fatalf("event[1] = %#v, want activity parent event", events[1])
	}
	if event, ok := events[3].(QSOEvent); !ok || event.Type != QSOEventType || event.Data.LogID != log.LogID {
		t.Fatalf("event[3] = %#v, want child qso event", events[3])
	}
}
