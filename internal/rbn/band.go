package rbn

func BandForFrequencyKHz(freq float64) (string, bool) {
	switch {
	case freq >= 135.7 && freq <= 137.8:
		return "2200m", true
	case freq >= 472 && freq <= 479:
		return "600m", true
	case freq >= 1800 && freq <= 2000:
		return "160m", true
	case freq >= 3500 && freq <= 4000:
		return "80m", true
	case freq >= 5300 && freq <= 5500:
		return "60m", true
	case freq >= 7000 && freq <= 7300:
		return "40m", true
	case freq >= 10100 && freq <= 10150:
		return "30m", true
	case freq >= 14000 && freq <= 14350:
		return "20m", true
	case freq >= 18068 && freq <= 18168:
		return "17m", true
	case freq >= 21000 && freq <= 21450:
		return "15m", true
	case freq >= 24890 && freq <= 24990:
		return "12m", true
	case freq >= 28000 && freq <= 30000:
		return "10m", true
	case freq >= 50000 && freq <= 54000:
		return "6m", true
	case freq >= 69900 && freq <= 70500:
		return "4m", true
	case freq >= 144000 && freq <= 148000:
		return "2m", true
	case freq >= 219000 && freq <= 225000:
		return "1.25m", true
	case freq >= 420000 && freq <= 450000:
		return "70cm", true
	case freq >= 902000 && freq <= 928000:
		return "33cm", true
	case freq >= 1240000 && freq <= 1300000:
		return "23cm", true
	default:
		return "", false
	}
}
