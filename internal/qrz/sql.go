package qrz

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const unknownString = "UNKNOWN"

const ProfileInsertSQL = `insert into qrz_callsigns (
  callsign,
  dxcc_id,
  dxcc_prefix,
  continent,
  country_name,
  qrz_ccode,
  first_name,
  last_name,
  state,
  county,
  grid,
  latitude,
  longitude,
  cq_zone,
  itu_zone,
  license_issue_date,
  license_exp_date,
  lookup_status,
  lookup_time
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func ProfileSQLArgs(profile Profile) []interface{} {
	return []interface{}{
		profile.Callsign,
		profile.DXCCID,
		defaultString(profile.DXCCPrefix),
		defaultString(profile.Continent),
		defaultString(profile.CountryName),
		profile.QRZCCode,
		profile.FirstName,
		profile.LastName,
		profile.State,
		profile.County,
		profile.Grid,
		defaultFloat(profile.Latitude),
		defaultFloat(profile.Longitude),
		profile.CQZone,
		profile.ITUZone,
		defaultDate(profile.LicenseIssueDate),
		defaultDate(profile.LicenseExpDate),
		defaultLookupStatus(profile.LookupStatus),
		formatSQLTime(profile.LookupTime),
	}
}

func defaultString(value string) string {
	if value == "" {
		return unknownString
	}
	return value
}

func defaultFloat(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func defaultDate(value string) string {
	if value == "" {
		return "1970-01-01 00:00:00"
	}
	if len(value) == len("2006-01-02") {
		return value + " 00:00:00"
	}
	return value
}

func defaultLookupStatus(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func formatSQLTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

type SQLStore struct {
	DB *sql.DB
}

func (s SQLStore) HasProfile(ctx context.Context, call string) (bool, error) {
	if s.DB == nil {
		return false, fmt.Errorf("nil sql db")
	}
	var status string
	err := s.DB.QueryRowContext(ctx, `select lookup_status from qrz_callsigns where callsign = ? limit 1`, normalizeCallsign(call)).Scan(&status)
	if err == nil {
		return true, nil
	}
	if errorsIsNoRows(err) {
		return false, nil
	}
	return false, err
}

func (s SQLStore) InsertProfile(ctx context.Context, profile Profile) error {
	if s.DB == nil {
		return fmt.Errorf("nil sql db")
	}
	_, err := s.DB.ExecContext(ctx, ProfileInsertSQL, ProfileSQLArgs(profile)...)
	return err
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
