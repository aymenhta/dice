package server

func elementAt[T any](slice []T, index int) (T, bool) {
	if index < 0 || index >= len(slice) {
		var zero T // Returns the zero value of the type (e.g., 0, "", false)
		return zero, false
	}
	return slice[index], true
}
