package server

import (
	"encoding/binary"
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

func lives(db HashMap, key []byte) bool {
	_, ok := db[string(key)]
	return ok
}

func wrongNumberOfArgumentErr(name string) resp.Frame {
	message := fmt.Sprintf("ERR wrong number of arguments for '%s' command", name)
	return resp.NewError(message)
}

func encodeInt64(n int64) []byte {
	b := make([]byte, 8)

	// Safe to cast: Two's Complement will handle the negative sign bits
	binary.BigEndian.PutUint64(b, uint64(n))
	return b
}
