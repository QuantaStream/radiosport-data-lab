package geo

import (
	"math"
	"testing"
)

func TestFromCTYCountryConvertsLongitudeConvention(t *testing.T) {
	tests := []struct {
		name         string
		lat          float64
		ctyLongitude float64
		wantLat      float64
		wantLong     float64
	}{
		{name: "Costa Rica west longitude", lat: 10, ctyLongitude: 84, wantLat: 10, wantLong: -84},
		{name: "Germany east longitude", lat: 51, ctyLongitude: -10, wantLat: 51, wantLong: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromCTYCountry(tt.lat, tt.ctyLongitude)
			if math.Abs(got.Latitude-tt.wantLat) > 0.000001 || math.Abs(got.Longitude-tt.wantLong) > 0.000001 {
				t.Fatalf("location = (%v,%v), want (%v,%v)", got.Latitude, got.Longitude, tt.wantLat, tt.wantLong)
			}
			if got.Source != SourceCTY || got.Confidence != ConfidenceCountryCentroid {
				t.Fatalf("provenance = %s/%s", got.Source, got.Confidence)
			}
		})
	}
}

func TestFromCTYCountryReturnsUnknownForEmptyCoordinates(t *testing.T) {
	got := FromCTYCountry(0, 0)
	if got != Unknown() {
		t.Fatalf("location = %+v, want unknown", got)
	}
}
