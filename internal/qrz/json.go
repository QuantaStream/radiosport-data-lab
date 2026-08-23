package qrz

import "time"

type ProfileEvent struct {
	Type string         `json:"type"`
	Data ProfilePayload `json:"data"`
}

type ProfilePayload struct {
	Callsign         string  `json:"callsign"`
	DXCCID           int     `json:"dxcc_id"`
	DXCCPrefix       string  `json:"dxcc_prefix"`
	Continent        string  `json:"continent"`
	CountryName      string  `json:"country_name"`
	QRZCCode         int     `json:"qrz_ccode"`
	FirstName        string  `json:"first_name"`
	LastName         string  `json:"last_name"`
	State            string  `json:"state"`
	County           string  `json:"county"`
	Grid             string  `json:"grid"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	CQZone           int     `json:"cq_zone"`
	ITUZone          int     `json:"itu_zone"`
	LicenseIssueDate string  `json:"license_issue_date"`
	LicenseExpDate   string  `json:"license_exp_date"`
	LookupStatus     string  `json:"lookup_status"`
	LookupTime       string  `json:"lookup_time"`
}

func NewProfileEvent(profile Profile) ProfileEvent {
	return ProfileEvent{
		Type: "qrz_callsign",
		Data: ProfilePayload{
			Callsign:         profile.Callsign,
			DXCCID:           profile.DXCCID,
			DXCCPrefix:       defaultString(profile.DXCCPrefix),
			Continent:        defaultString(profile.Continent),
			CountryName:      defaultString(profile.CountryName),
			QRZCCode:         profile.QRZCCode,
			FirstName:        profile.FirstName,
			LastName:         profile.LastName,
			State:            profile.State,
			County:           profile.County,
			Grid:             profile.Grid,
			Latitude:         defaultFloat(profile.Latitude),
			Longitude:        defaultFloat(profile.Longitude),
			CQZone:           profile.CQZone,
			ITUZone:          profile.ITUZone,
			LicenseIssueDate: defaultDate(profile.LicenseIssueDate),
			LicenseExpDate:   defaultDate(profile.LicenseExpDate),
			LookupStatus:     defaultLookupStatus(profile.LookupStatus),
			LookupTime:       formatSQLTime(profile.LookupTime),
		},
	}
}

func NewPendingProfileEvent(call string) ProfileEvent {
	return NewProfileEvent(PendingProfile(call, time.Time{}))
}
