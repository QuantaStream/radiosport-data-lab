package geo

const (
	SourceUnknown = "UNKNOWN"
	SourceCTY     = "CTY_COUNTRY"

	ConfidenceUnknown         = "UNKNOWN"
	ConfidenceCountryCentroid = "COUNTRY_CENTROID"
)

type Location struct {
	Latitude   float64
	Longitude  float64
	Source     string
	Confidence string
}

func Unknown() Location {
	return Location{
		Source:     SourceUnknown,
		Confidence: ConfidenceUnknown,
	}
}

func FromCTYCountry(latitude float64, ctyLongitude float64) Location {
	if latitude == 0 && ctyLongitude == 0 {
		return Unknown()
	}
	return Location{
		Latitude:   latitude,
		Longitude:  -ctyLongitude,
		Source:     SourceCTY,
		Confidence: ConfidenceCountryCentroid,
	}
}
