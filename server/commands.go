// if the error is a client error, then send it to them
// otherwise return the error to the caller

package server

import (
	"aymenhta.com/dice/resp"
)

func Command() resp.Frame {
	return resp.NewOkSimpleString()
}

func set(db DB, key, value []byte) resp.Frame {
	db[string(key)] = value
	return resp.NewOkSimpleString()
}

func get(db DB, key []byte) resp.Frame {
	value, ok := db[string(key)]
	if !ok {
		return resp.NewNullBulkString()
	}

	return resp.NewBulkString(value)
}
