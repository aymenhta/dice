// if the error is a client error, then send it to them
// otherwise return the error to the caller

package server

import (
	"fmt"
	"strconv"

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

func del(db DB, keys [][]byte) resp.Frame {
	var count int64
	for _, key := range keys {
		if lives(db, key) {
			delete(db, string(key))
			count = count + 1
		}
	}

	return &resp.Integer{Value: strconv.AppendInt(nil, count, 10)}
}

func exists(db DB, keys [][]byte) resp.Frame {
	var count int64
	for _, key := range keys {
		if lives(db, key) {
			count = count + 1
		}
	}

	return &resp.Integer{Value: strconv.AppendInt(nil, count, 10)}
}

func incr(db DB, key []byte, delta int64) resp.Frame {
	var current int64
	v, ok := db[string(key)]

	if ok {
		n, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil {
			return resp.NewError("ERR value is not an integer or out of range")
		}

		current = n
	}

	next := strconv.AppendInt(nil, current+delta, 10)
	db[string(key)] = next
	res := &resp.Integer{Value: next}
	fmt.Println(res)
	return res
}

func selectCMD(index []byte, currentDB *int) resp.Frame {
	n, err := strconv.Atoi(string(index))
	if err == nil && n > 0 && n < MAX_DB_NUM {
		*currentDB = n
		return resp.NewOkSimpleString()
	}

	return resp.NewError("ERR DB index is out of range")
}
