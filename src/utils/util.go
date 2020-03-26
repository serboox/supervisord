package utils

// FillIfEmpty fill target string if it is empty
func FillIfEmpty(target *string, defaultValue string) {
	if *target == "" {
		*target = defaultValue
	}
}

// InArray return true if the elem is in the array arr
func InArray(elem interface{}, arr []interface{}) bool {
	for _, e := range arr {
		if e == elem {
			return true
		}
	}

	return false
}
