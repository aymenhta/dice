package server

func elementAt[T any](slice []T, index int) T {
	if index < 0 || index >= len(slice) {
		var zero T // Returns the zero value of the type (e.g., 0, "", false)
		return zero
	}
	return slice[index]
}
