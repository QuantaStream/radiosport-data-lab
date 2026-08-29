package cabrillo

import (
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

const (
	LogEventType = "contest_log"
	QSOEventType = "contest_qso"
)

type LogEvent struct {
	Type string     `json:"type"`
	Data LogPayload `json:"data"`
}

type LogPayload struct {
	LogID                string  `json:"log_id"`
	ContestID            string  `json:"contest_id"`
	StationCall          string  `json:"station_call"`
	StationPrefix        string  `json:"station_prefix"`
	StationContinent     string  `json:"station_continent"`
	StationCountry       string  `json:"station_country"`
	CQZone               int     `json:"cq_zone"`
	ITUZone              int     `json:"itu_zone"`
	StationLatitude      float64 `json:"station_latitude"`
	StationLongitude     float64 `json:"station_longitude"`
	StationGeoSource     string  `json:"station_geo_source"`
	StationGeoConfidence string  `json:"station_geo_confidence"`
	CategoryOperator     string  `json:"category_operator"`
	CategoryAssisted     string  `json:"category_assisted"`
	CategoryBand         string  `json:"category_band"`
	CategoryPower        string  `json:"category_power"`
	CategoryMode         string  `json:"category_mode"`
	CategoryTransmitter  string  `json:"category_transmitter"`
	ClaimedScore         int64   `json:"claimed_score"`
	QSOCount             int     `json:"qso_count"`
	ScopeRegion          string  `json:"scope_region"`
	SourceFile           string  `json:"source_file"`
	LoadedAt             string  `json:"loaded_at"`
}

type QSOEvent struct {
	Type string     `json:"type"`
	Data QSOPayload `json:"data"`
}

type QSOPayload struct {
	QSOID            uint64  `json:"qso_id"`
	LogID            string  `json:"log_id"`
	ContestID        string  `json:"contest_id"`
	QSOAt            string  `json:"qso_at"`
	QSODayKey        int     `json:"qso_day_key"`
	QSO3HBucketKey   int     `json:"qso_3h_bucket_key"`
	QSO5MBucketKey   int     `json:"qso_5m_bucket_key"`
	Activity5MID     uint64  `json:"activity_5m_id"`
	Activity5MKey    string  `json:"activity_5m_key"`
	StationCall      string  `json:"station_call"`
	StationPrefix    string  `json:"station_prefix"`
	StationContinent string  `json:"station_continent"`
	WorkedCall       string  `json:"worked_call"`
	WorkedPrefix     string  `json:"worked_prefix"`
	WorkedContinent  string  `json:"worked_continent"`
	FrequencyKHz     float64 `json:"frequency_khz"`
	Band             string  `json:"band"`
	Mode             string  `json:"mode"`
	SentExchange     string  `json:"sent_exchange"`
	ReceivedExchange string  `json:"received_exchange"`
	SourceFile       string  `json:"source_file"`
}

func NewLogEvent(log Log) LogEvent {
	return LogEvent{
		Type: LogEventType,
		Data: LogPayload{
			LogID:                log.LogID,
			ContestID:            log.ContestID,
			StationCall:          log.StationCall,
			StationPrefix:        log.StationPrefix,
			StationContinent:     log.StationContinent,
			StationCountry:       log.StationCountry,
			CQZone:               log.CQZone,
			ITUZone:              log.ITUZone,
			StationLatitude:      log.StationLatitude,
			StationLongitude:     log.StationLongitude,
			StationGeoSource:     log.StationGeoSource,
			StationGeoConfidence: log.StationGeoConfidence,
			CategoryOperator:     log.CategoryOperator,
			CategoryAssisted:     log.CategoryAssisted,
			CategoryBand:         log.CategoryBand,
			CategoryPower:        log.CategoryPower,
			CategoryMode:         log.CategoryMode,
			CategoryTransmitter:  log.CategoryTransmitter,
			ClaimedScore:         log.ClaimedScore,
			QSOCount:             log.QSOCount,
			ScopeRegion:          log.ScopeRegion,
			SourceFile:           log.SourceFile,
			LoadedAt:             formatTime(log.LoadedAt),
		},
	}
}

func NewQSOEvent(qso QSO) QSOEvent {
	return QSOEvent{
		Type: QSOEventType,
		Data: QSOPayload{
			QSOID:            qso.QSOID,
			LogID:            qso.LogID,
			ContestID:        qso.ContestID,
			QSOAt:            formatTime(qso.QSOAt),
			QSODayKey:        qso.QSODayKey,
			QSO3HBucketKey:   qso.QSO3HBucketKey,
			QSO5MBucketKey:   qso.QSO5MBucketKey,
			Activity5MID:     qso.Activity5MID,
			Activity5MKey:    qso.Activity5MKey,
			StationCall:      qso.StationCall,
			StationPrefix:    qso.StationPrefix,
			StationContinent: qso.StationContinent,
			WorkedCall:       qso.WorkedCall,
			WorkedPrefix:     qso.WorkedPrefix,
			WorkedContinent:  qso.WorkedContinent,
			FrequencyKHz:     qso.FrequencyKHz,
			Band:             qso.Band,
			Mode:             qso.Mode,
			SentExchange:     qso.SentExchange,
			ReceivedExchange: qso.ReceivedExchange,
			SourceFile:       qso.SourceFile,
		},
	}
}

func NewEvents(log Log, qsos []QSO) []interface{} {
	return NewEventsWithActivityParents(log, qsos, true)
}

func NewEventsWithActivityParents(log Log, qsos []QSO, activityParents bool) []interface{} {
	var bucketEvents []interface{}
	if activityParents {
		bucketEvents = NewActivity5MBucketEvents(qsos)
	}
	events := make([]interface{}, 0, len(qsos)+len(bucketEvents)+1)
	events = append(events, NewLogEvent(log))
	events = append(events, bucketEvents...)
	for _, qso := range qsos {
		events = append(events, NewQSOEvent(qso))
	}
	return events
}

func NewActivity5MBucketEvents(qsos []QSO) []interface{} {
	buckets := map[uint64]rbn.Activity5MBucket{}
	for _, qso := range qsos {
		bucket := rbn.NewActivity5MBucket(qso.StationCall, qso.Band, qso.Mode, qso.QSOAt)
		buckets[bucket.Activity5MID] = bucket
	}
	ordered := rbn.SortedActivity5MBuckets(buckets)
	events := make([]interface{}, 0, len(ordered))
	for _, bucket := range ordered {
		events = append(events, rbn.NewActivity5MBucketEvent(bucket))
	}
	return events
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
