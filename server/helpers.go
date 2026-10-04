package server

import (
	"fmt"

	"aymenhta.com/dice/resp"
)

func elementAt[T any](slice []T, index int) T {
	if index < 0 || index >= len(slice) {
		var zero T // Returns the zero value of the type (e.g., 0, "", false)
		return zero
	}
	return slice[index]
}

func lives(db DB, key []byte) bool {
	_, ok := db[string(key)]
	return ok
}

func wrongNumberOfArgumentErr(name string) resp.Frame {
	message := fmt.Sprintf("ERR wrong number of arguments for '%s' command", name)
	return resp.NewError(message)
}
