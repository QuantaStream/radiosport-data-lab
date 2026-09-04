package callsign

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Database struct {
	countriesByPrefix map[string]CountryInfo
	aliases           map[string]PrefixAlias
	exact             map[string]PrefixAlias
}

type Station struct {
	Valid         bool
	Call          string
	Prefix        string
	PrimaryPrefix string
	Homecall      string
	Country       string
	Latitude      float64
	Longitude     float64
	CQZone        int
	ITUZone       int
	Continent     string
	Offset        float64
	Maritime      bool
	Aeronautical  bool
	Beacon        bool
	CallArea      string
}

type CountryInfo struct {
	Country       string
	CQZone        int
	ITUZone       int
	Continent     string
	Latitude      float64
	Longitude     float64
	Offset        float64
	PrimaryPrefix string
}

type PrefixAlias struct {
	Prefix  string
	CQZone  int
	ITUZone int
	Parent  CountryInfo
}

var (
	reEndPrefix        = regexp.MustCompile(`[([]`)
	reGetCQZone        = regexp.MustCompile(`[(]([0-9]+)[)]`)
	reGetITUZone       = regexp.MustCompile(`[[]([0-9]+)[]]`)
	reHasThreeChars    = regexp.MustCompile(`(?i)[/A-Z0-9\-]{3,16}`)
	reLeadingNumber    = regexp.MustCompile(`(?i)^[0-9][A-Z]{1,2}?([0-9])[A-Z0-9]+$`)
	reLeadingAlpha     = regexp.MustCompile(`(?i)^[A-Z]{1,2}?([0-9]{1,4})[A-Z0-9]+$`)
	reRemoveDashSuffix = regexp.MustCompile(`(?i)-[0-9#-]{1,4}$`)
)

func LoadFile(path string) (*Database, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Load(file)
}

func LoadDefault() (*Database, string, error) {
	if path := strings.TrimSpace(os.Getenv("RBN_CTY_DAT")); path != "" {
		db, err := LoadFile(path)
		return db, path, err
	}
	path, ok := findDefaultCTYPath()
	if !ok {
		return nil, "", os.ErrNotExist
	}
	db, err := LoadFile(path)
	return db, path, err
}

func Load(r io.Reader) (*Database, error) {
	db := &Database{
		countriesByPrefix: map[string]CountryInfo{},
		aliases:           map[string]PrefixAlias{},
		exact:             map[string]PrefixAlias{},
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var current CountryInfo
	haveCountry := false
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ":")
		if len(fields) == 9 {
			country, err := parseCountryFields(fields)
			if err != nil {
				return nil, err
			}
			current = country
			haveCountry = true
			continue
		}
		if !haveCountry {
			continue
		}
		lastLine := strings.HasSuffix(strings.TrimRight(line, " \t"), ";")
		line = strings.TrimSuffix(strings.TrimSpace(line), ";")
		for _, token := range strings.Split(line, ",") {
			if err := db.addAlias(current, token); err != nil {
				return nil, err
			}
		}
		if lastLine {
			db.countriesByPrefix[normalizePrefix(current.PrimaryPrefix)] = current
			haveCountry = false
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(db.aliases) == 0 && len(db.exact) == 0 {
		return nil, fmt.Errorf("cty file contained no aliases")
	}
	return db, nil
}

func (db *Database) Parse(call string) (Station, error) {
	if db == nil {
		return Station{}, fmt.Errorf("nil callsign database")
	}
	st := Station{Call: strings.ToUpper(strings.TrimSpace(call))}
	st.parseCall(db, st.Call)
	if !st.Valid {
		return st, fmt.Errorf("callsign %q could not be decoded", st.Call)
	}
	if st.Maritime || st.Aeronautical || st.Prefix == "" {
		return st, nil
	}
	if alias, ok := db.exact[normalizePrefix(st.Homecall)]; ok {
		applyAlias(&st, alias)
		return st, nil
	}
	alias, ok := db.LookupPrefix(st.Prefix)
	if !ok {
		st.Valid = false
		return st, fmt.Errorf("no country info found for callsign %q prefix %q", st.Call, st.Prefix)
	}
	applyAlias(&st, alias)
	return st, nil
}

func applyAlias(st *Station, alias PrefixAlias) {
	st.Country = alias.Parent.Country
	st.Latitude = alias.Parent.Latitude
	st.Longitude = alias.Parent.Longitude
	st.PrimaryPrefix = alias.Parent.PrimaryPrefix
	st.CQZone = alias.CQZone
	st.ITUZone = alias.ITUZone
	st.Continent = alias.Parent.Continent
	st.Offset = alias.Parent.Offset
}

func (db *Database) LookupPrefix(prefix string) (PrefixAlias, bool) {
	alias, ok := db.aliases[normalizePrefix(prefix)]
	return alias, ok
}

func (db *Database) addAlias(country CountryInfo, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	exact := strings.HasPrefix(token, "=")
	if exact {
		token = strings.TrimPrefix(token, "=")
	}
	token = strings.TrimPrefix(token, "*")
	idx := reEndPrefix.FindStringIndex(token)
	prefix := token
	if len(idx) > 0 {
		prefix = token[:idx[0]]
	}
	prefix = normalizePrefix(prefix)
	if prefix == "" {
		return nil
	}
	alias := PrefixAlias{
		Prefix:  prefix,
		CQZone:  country.CQZone,
		ITUZone: country.ITUZone,
		Parent:  country,
	}
	if match := reGetCQZone.FindStringSubmatch(token); match != nil {
		zone, err := strconv.Atoi(match[1])
		if err != nil {
			return err
		}
		alias.CQZone = zone
	}
	if match := reGetITUZone.FindStringSubmatch(token); match != nil {
		zone, err := strconv.Atoi(match[1])
		if err != nil {
			return err
		}
		alias.ITUZone = zone
	}
	if exact {
		db.exact[prefix] = alias
		return nil
	}
	db.aliases[prefix] = alias
	return nil
}

func (st *Station) parseCall(db *Database, call string) {
	if reHasThreeChars.FindString(call) == "" {
		return
	}
	call = reRemoveDashSuffix.ReplaceAllString(call, "")
	if alias, ok := db.exact[normalizePrefix(call)]; ok {
		st.Valid = true
		st.Homecall = call
		st.Prefix = alias.Prefix
		return
	}
	segments := strings.Split(call, "/")
	switch len(segments) {
	case 1:
		st.checkCall(db, call, call)
	case 2:
		hasDesig := st.checkDesignator(segments[1])
		if _, err := strconv.Atoi(segments[1]); !hasDesig && len(segments[1]) == 1 && err == nil {
			st.CallArea = segments[1]
		}
		if hasDesig || st.CallArea != "" {
			st.checkCall(db, segments[0], segments[0])
			if st.Maritime || st.Aeronautical {
				st.Prefix = ""
			}
			return
		}
		okCall1, okPrefix1 := st.checkCall(db, segments[0], segments[1])
		if okCall1 && okPrefix1 {
			if okCall2, okPrefix2 := st.checkCall(db, segments[1], segments[0]); okCall2 && okPrefix2 {
				if prefix, ok := db.iteratePrefix(segments[1]); ok && st.Homecall == prefix {
					st.Prefix = segments[1]
					st.Homecall = segments[0]
				}
				return
			}
			st.Homecall = segments[0]
			if prefix, ok := db.iteratePrefix(segments[1]); ok {
				st.Prefix = prefix
			}
			return
		}
		st.checkCall(db, segments[1], segments[0])
	case 3:
		st.checkDesignator(segments[2])
		var callArea string
		if segments[2] == "P" {
			if _, err := strconv.Atoi(segments[1]); err == nil {
				callArea = segments[1]
			}
		}
		if (segments[2] == "P" && segments[1] == "LH") || (segments[1] == "P" && segments[2] == "LH") {
			st.checkCall(db, segments[0], segments[0])
			return
		}
		if st.Maritime || st.Aeronautical {
			st.Valid = false
			return
		}
		if okCall, okPrefix := st.checkCall(db, segments[1], segments[0]); okCall && okPrefix {
			return
		}
		okCall, okPrefix := st.checkCall(db, segments[0], segments[1])
		if okCall && okPrefix {
			return
		}
		if okCall && !okPrefix && callArea != "" && segments[2] == "P" {
			st.CallArea = callArea
			st.Valid = true
			st.Homecall = segments[0]
		}
	}
}

func (st *Station) checkCall(db *Database, call string, prefixInput string) (bool, bool) {
	validCall := false
	var callArea string
	if match := reLeadingAlpha.FindStringSubmatch(call); match != nil {
		if len(match[1]) <= 2 {
			callArea = match[1][len(match[1])-1:]
		}
		validCall = true
	} else if match := reLeadingNumber.FindStringSubmatch(call); match != nil {
		callArea = match[1][len(match[1])-1:]
		validCall = true
	}
	prefix, validPrefix := db.iteratePrefix(prefixInput)
	if validCall {
		st.Homecall = call
	}
	if validPrefix {
		st.Prefix = prefix
	}
	if validCall && validPrefix {
		st.Valid = true
		if st.CallArea == "" {
			st.CallArea = callArea
		}
	}
	return validCall, validPrefix
}

func (st *Station) checkDesignator(s string) bool {
	switch s {
	case "MM":
		st.Maritime = true
		return true
	case "AM":
		st.Aeronautical = true
		return true
	case "BCN", "B":
		st.Beacon = true
		return true
	case "A", "LH", "M", "P", "QRP", "QRPP":
		return true
	default:
		return false
	}
}

func (db *Database) iteratePrefix(call string) (string, bool) {
	prefix := normalizePrefix(call)
	for len(prefix) > 0 {
		if _, found := db.aliases[prefix]; found {
			return prefix, true
		}
		prefix = prefix[:len(prefix)-1]
	}
	return "", false
}

func parseCountryFields(fields []string) (CountryInfo, error) {
	country := CountryInfo{
		Country:       strings.TrimSpace(fields[0]),
		Continent:     strings.ToUpper(strings.TrimSpace(fields[3])),
		PrimaryPrefix: normalizePrefix(fields[7]),
	}
	var err error
	if country.CQZone, err = parseIntField(fields[1]); err != nil {
		return CountryInfo{}, err
	}
	if country.ITUZone, err = parseIntField(fields[2]); err != nil {
		return CountryInfo{}, err
	}
	if country.Latitude, err = parseFloatField(fields[4]); err != nil {
		return CountryInfo{}, err
	}
	if country.Longitude, err = parseFloatField(fields[5]); err != nil {
		return CountryInfo{}, err
	}
	if country.Offset, err = parseFloatField(fields[6]); err != nil {
		return CountryInfo{}, err
	}
	if country.Country == "" || country.PrimaryPrefix == "" {
		return CountryInfo{}, fmt.Errorf("invalid country row %q", strings.Join(fields, ":"))
	}
	return country, nil
}

func parseIntField(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}

func parseFloatField(value string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(value), 64)
}

func normalizePrefix(input string) string {
	return strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(input, "*")))
}

func findDefaultCTYPath() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(wd, "data", "cty", "cty.dat")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", false
		}
		wd = parent
	}
}
