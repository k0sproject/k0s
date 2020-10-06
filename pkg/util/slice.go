package util

// StringSliceContains check whether the given string slice contains the other string
func StringSliceContains(strSlice []string, str string) bool {
	for _, s := range strSlice {
		if s == str {
			return true
		}
	}

	return false
}
