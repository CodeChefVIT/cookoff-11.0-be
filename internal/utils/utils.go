package utils

func GetExecutionTimeMultiplier(languageID int) float64 {
	switch languageID {
	case 50, 54, 60, 73, 63:
		return 1
	case 51, 62:
		return 2
	case 68:
		return 3
	case 71:
		return 5
	default:
		return 0
	}

}
