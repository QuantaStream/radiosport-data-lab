package qrz

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://xmldata.qrz.com/xml/current/"

var (
	ErrNotConfigured = errors.New("qrz credentials are not configured")
	ErrNotFound      = errors.New("qrz callsign not found")
	errSessionGone   = errors.New("qrz session expired")
)

type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
	session  string
	now      func() time.Time
	notFound map[string]struct{}
}

type Option func(*Client)

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSpace(baseURL)
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.http = client
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(c *Client) {
		if now != nil {
			c.now = now
		}
	}
}

func NewClient(username, password string, opts ...Option) *Client {
	c := &Client{
		baseURL:  DefaultBaseURL,
		username: strings.TrimSpace(username),
		password: strings.TrimSpace(password),
		http:     &http.Client{Timeout: 5 * time.Second},
		now:      time.Now,
		notFound: map[string]struct{}{},
	}
	if envBase := strings.TrimSpace(os.Getenv("QRZ_BASE_URL")); envBase != "" {
		c.baseURL = envBase
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func NewClientFromEnv(opts ...Option) (*Client, error) {
	username := strings.TrimSpace(os.Getenv("QRZ_USERNAME"))
	password := strings.TrimSpace(os.Getenv("QRZ_PASSWORD"))
	if username == "" || password == "" {
		return nil, ErrNotConfigured
	}
	return NewClient(username, password, opts...), nil
}

type Profile struct {
	Callsign         string    `json:"callsign"`
	DXCCID           int       `json:"dxcc_id"`
	DXCCPrefix       string    `json:"dxcc_prefix"`
	Continent        string    `json:"continent"`
	CountryName      string    `json:"country_name"`
	QRZCCode         int       `json:"qrz_ccode"`
	FirstName        string    `json:"first_name"`
	LastName         string    `json:"last_name"`
	State            string    `json:"state"`
	County           string    `json:"county"`
	Grid             string    `json:"grid"`
	Latitude         *float64  `json:"latitude,omitempty"`
	Longitude        *float64  `json:"longitude,omitempty"`
	CQZone           int       `json:"cq_zone"`
	ITUZone          int       `json:"itu_zone"`
	LicenseIssueDate string    `json:"license_issue_date,omitempty"`
	LicenseExpDate   string    `json:"license_exp_date,omitempty"`
	LookupStatus     string    `json:"lookup_status"`
	LookupTime       time.Time `json:"lookup_time"`
}

func NotFoundProfile(call string, lookupTime time.Time) Profile {
	return Profile{
		Callsign:     normalizeCallsign(call),
		DXCCPrefix:   unknownString,
		Continent:    unknownString,
		CountryName:  unknownString,
		LookupStatus: "not_found",
		LookupTime:   lookupTime.UTC(),
	}
}

func (c *Client) Lookup(ctx context.Context, call string) (Profile, error) {
	if strings.TrimSpace(c.username) == "" || strings.TrimSpace(c.password) == "" {
		return Profile{}, ErrNotConfigured
	}
	call = normalizeCallsign(call)
	if call == "" {
		return Profile{}, fmt.Errorf("empty callsign")
	}
	if _, ok := c.notFound[call]; ok {
		return Profile{}, ErrNotFound
	}
	if c.session == "" {
		if err := c.login(ctx); err != nil {
			return Profile{}, err
		}
	}
	profile, err := c.lookupOnce(ctx, call)
	if errors.Is(err, errSessionGone) {
		c.session = ""
		if err := c.login(ctx); err != nil {
			return Profile{}, err
		}
		profile, err = c.lookupOnce(ctx, call)
	}
	if errors.Is(err, ErrNotFound) {
		c.notFound[call] = struct{}{}
	}
	return profile, err
}

func (c *Client) login(ctx context.Context) error {
	resp, err := c.request(ctx, map[string]string{
		"username": c.username,
		"password": c.password,
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(resp.Session.Key) == "" {
		if resp.Session.Error != "" {
			return fmt.Errorf("qrz login failed: %s", resp.Session.Error)
		}
		return fmt.Errorf("qrz login failed: no session key returned")
	}
	c.session = strings.TrimSpace(resp.Session.Key)
	return nil
}

func (c *Client) lookupOnce(ctx context.Context, call string) (Profile, error) {
	resp, err := c.request(ctx, map[string]string{
		"s":        c.session,
		"callsign": call,
	})
	if err != nil {
		return Profile{}, err
	}
	if strings.TrimSpace(resp.Callsign.Call) == "" {
		return Profile{}, ErrNotFound
	}
	return resp.profile(c.now().UTC()), nil
}

func (c *Client) request(ctx context.Context, params map[string]string) (apiResponse, error) {
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return apiResponse{}, err
	}
	query := base.Query()
	for k, v := range params {
		query.Set(k, v)
	}
	base.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return apiResponse{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		return apiResponse{}, fmt.Errorf("qrz request failed: %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return apiResponse{}, err
	}
	var parsed apiResponse
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return apiResponse{}, err
	}
	if err := classifySessionError(parsed.Session.Error); err != nil {
		return apiResponse{}, err
	}
	return parsed, nil
}

type apiResponse struct {
	XMLName  xml.Name    `xml:"QRZDatabase"`
	Session  apiSession  `xml:"Session"`
	Callsign apiCallsign `xml:"Callsign"`
}

type apiSession struct {
	Key     string `xml:"Key"`
	SubExp  string `xml:"SubExp"`
	GMTime  string `xml:"GMTime"`
	Count   int    `xml:"Count"`
	Message string `xml:"Message"`
	Remark  string `xml:"Remark"`
	Error   string `xml:"Error"`
}

type apiCallsign struct {
	Call    string `xml:"call"`
	Aliases string `xml:"aliases"`
	DXCC    string `xml:"dxcc"`
	First   string `xml:"fname"`
	Last    string `xml:"name"`
	State   string `xml:"state"`
	County  string `xml:"county"`
	Grid    string `xml:"grid"`
	Lat     string `xml:"lat"`
	Lon     string `xml:"lon"`
	CCode   string `xml:"ccode"`
	Country string `xml:"country"`
	Land    string `xml:"land"`
	CQZone  string `xml:"cqzone"`
	ITUZone string `xml:"ituzone"`
	EFDate  string `xml:"efdate"`
	ExpDate string `xml:"expdate"`
}

func (resp apiResponse) profile(lookupTime time.Time) Profile {
	cs := resp.Callsign
	countryName := firstNonEmpty(cs.Land, cs.Country, unknownString)
	return Profile{
		Callsign:         normalizeCallsign(cs.Call),
		DXCCID:           parseInt(cs.DXCC),
		DXCCPrefix:       unknownString,
		Continent:        unknownString,
		CountryName:      countryName,
		QRZCCode:         parseInt(cs.CCode),
		FirstName:        strings.TrimSpace(cs.First),
		LastName:         strings.TrimSpace(cs.Last),
		State:            strings.TrimSpace(cs.State),
		County:           strings.TrimSpace(cs.County),
		Grid:             strings.ToUpper(strings.TrimSpace(cs.Grid)),
		Latitude:         parseOptionalFloat(cs.Lat),
		Longitude:        parseOptionalFloat(cs.Lon),
		CQZone:           parseInt(cs.CQZone),
		ITUZone:          parseInt(cs.ITUZone),
		LicenseIssueDate: normalizeDate(cs.EFDate),
		LicenseExpDate:   normalizeDate(cs.ExpDate),
		LookupStatus:     "found",
		LookupTime:       lookupTime.UTC(),
	}
}

func classifySessionError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "session timeout"):
		return fmt.Errorf("%w: %s", errSessionGone, message)
	case strings.Contains(lower, "not found"):
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	default:
		return errors.New(message)
	}
}

func normalizeCallsign(input string) string {
	return strings.ToUpper(strings.TrimSpace(input))
}

func parseInt(input string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(input))
	return n
}

func parseOptionalFloat(input string) *float64 {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	n, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return nil
	}
	return &n
}

func normalizeDate(input string) string {
	input = strings.TrimSpace(input)
	if input == "" || input == "0000-00-00" {
		return ""
	}
	return input
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
