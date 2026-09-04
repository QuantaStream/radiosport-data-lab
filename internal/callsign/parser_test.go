package callsign

import "testing"

func loadTestDB(t *testing.T) *Database {
	t.Helper()
	db, err := LoadFile("testdata/cty.dat")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestParseCommonCallsigns(t *testing.T) {
	db := loadTestDB(t)
	tests := []struct {
		call      string
		prefix    string
		primary   string
		continent string
		country   string
	}{
		{call: "DH1TW/P", prefix: "DH", primary: "DL", continent: "EU", country: "Federal Republic of Germany"},
		{call: "HC2/DH1TW/P", prefix: "HC", primary: "HC", continent: "SA", country: "Ecuador"},
		{call: "DH1TW/VP5", prefix: "VP5", primary: "VP5", continent: "NA", country: "Turks & Caicos Islands"},
		{call: "VP2E/DL2001IRTA/P", prefix: "VP2E", primary: "VP2M", continent: "NA", country: "Montserrat"},
		{call: "W3LPL/5", prefix: "W", primary: "K", continent: "NA", country: "United States"},
		{call: "UA9MAT/1", prefix: "UA9", primary: "UA9", continent: "AS", country: "Asiatic Russia"},
		{call: "8J3XVIII", prefix: "8J", primary: "JA", continent: "AS", country: "Japan"},
		{call: "8J2025XYZ", prefix: "8J", primary: "JA", continent: "AS", country: "Japan"},
		{call: "UR5ZEP/A", prefix: "UR", primary: "UR", continent: "EU", country: "Ukraine"},
		{call: "GW8IZR-#", prefix: "GW", primary: "G", continent: "EU", country: "England"},
	}
	for _, tt := range tests {
		t.Run(tt.call, func(t *testing.T) {
			station, err := db.Parse(tt.call)
			if err != nil {
				t.Fatal(err)
			}
			if station.Prefix != tt.prefix || station.PrimaryPrefix != tt.primary || station.Continent != tt.continent || station.Country != tt.country {
				t.Fatalf("station = %+v", station)
			}
		})
	}
}

func TestParseExactCallsignOverride(t *testing.T) {
	db := loadTestDB(t)
	station, err := db.Parse("4U1UN")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := station.PrimaryPrefix, "4U1U"; got != want {
		t.Fatalf("primary prefix = %q, want %q", got, want)
	}
}

func TestParseMaritimeMobileKeepsValidButNoCountry(t *testing.T) {
	db := loadTestDB(t)
	station, err := db.Parse("DH1TW/MM")
	if err != nil {
		t.Fatal(err)
	}
	if !station.Valid || !station.Maritime || station.Prefix != "" || station.PrimaryPrefix != "" {
		t.Fatalf("station = %+v", station)
	}
}

func TestInvalidCallsign(t *testing.T) {
	db := loadTestDB(t)
	if _, err := db.Parse("CQ"); err == nil {
		t.Fatal("expected invalid callsign error")
	}
}
