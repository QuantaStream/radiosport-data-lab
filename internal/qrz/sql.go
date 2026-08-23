package qrz

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const unknownString = "UNKNOWN"
const PendingLookupStatus = "pending"

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

const ProfileUpdateSQL = `update qrz_callsigns set
  dxcc_id = ?,
  dxcc_prefix = ?,
  continent = ?,
  country_name = ?,
  qrz_ccode = ?,
  first_name = ?,
  last_name = ?,
  state = ?,
  county = ?,
  grid = ?,
  latitude = ?,
  longitude = ?,
  cq_zone = ?,
  itu_zone = ?,
  license_issue_date = ?,
  license_exp_date = ?,
  lookup_status = ?,
  lookup_time = ?
where callsign = ?`

func PendingProfile(call string, lookupTime time.Time) Profile {
	return Profile{
		Callsign:     normalizeCallsign(call),
		DXCCPrefix:   unknownString,
		Continent:    unknownString,
		CountryName:  unknownString,
		LookupStatus: PendingLookupStatus,
		LookupTime:   lookupTime.UTC(),
	}
}

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

func ProfileUpdateSQLArgs(profile Profile) []interface{} {
	args := ProfileSQLArgs(profile)
	if len(args) == 0 {
		return args
	}
	return append(args[1:], profile.Callsign)
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

func (s SQLStore) LookupStatus(ctx context.Context, call string) (string, bool, error) {
	if s.DB == nil {
		return "", false, fmt.Errorf("nil sql db")
	}
	var status string
	err := s.DB.QueryRowContext(ctx, `select lookup_status from qrz_callsigns where callsign = ? limit 1`, normalizeCallsign(call)).Scan(&status)
	if err == nil {
		return status, true, nil
	}
	if errorsIsNoRows(err) {
		return "", false, nil
	}
	return "", false, err
}

func (s SQLStore) HasProfile(ctx context.Context, call string) (bool, error) {
	_, ok, err := s.LookupStatus(ctx, call)
	return ok, err
}

func (s SQLStore) EnsurePendingProfile(ctx context.Context, call string) (bool, error) {
	if s.DB == nil {
		return false, fmt.Errorf("nil sql db")
	}
	call = normalizeCallsign(call)
	if call == "" {
		return false, nil
	}
	if _, ok, err := s.LookupStatus(ctx, call); err != nil || ok {
		return false, err
	}
	if err := s.InsertProfile(ctx, PendingProfile(call, time.Now())); err != nil {
		return false, err
	}
	return true, nil
}

func (s SQLStore) InsertProfile(ctx context.Context, profile Profile) error {
	if s.DB == nil {
		return fmt.Errorf("nil sql db")
	}
	_, err := s.DB.ExecContext(ctx, ProfileInsertSQL, ProfileSQLArgs(profile)...)
	return err
}

func (s SQLStore) UpdateProfile(ctx context.Context, profile Profile) error {
	if s.DB == nil {
		return fmt.Errorf("nil sql db")
	}
	_, err := s.DB.ExecContext(ctx, ProfileUpdateSQL, ProfileUpdateSQLArgs(profile)...)
	return err
}

func (s SQLStore) StoreProfile(ctx context.Context, profile Profile) error {
	if _, err := s.EnsurePendingProfile(ctx, profile.Callsign); err != nil {
		return err
	}
	return s.UpdateProfile(ctx, profile)
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}
