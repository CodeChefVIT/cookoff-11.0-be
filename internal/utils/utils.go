package utils


//map ids to multiplier to multipliers here
func GetRuntimeMultiplier(languageID int) float64 {
	switch languageID {
	case 1:
		return 1.0
	default:
		return 0.0
	}
}