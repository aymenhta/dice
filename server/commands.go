// if the error is a client error, then send it to them
// otherwise return the error to the caller

package server

import (
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
	count := 0
	for _, key := range keys {
		sk := string(key)
		if _, ok := db[sk]; ok {
			delete(db, sk)
			count = count + 1
		}
	}

	return &resp.Integer{Value: []byte(strconv.Itoa(count))}
}

func selectCMD(index []byte, currentDB *int) resp.Frame {
	n, err := strconv.Atoi(string(index))
	if err == nil && n > 0 && n < MAX_DB_NUM {
		*currentDB = n
		return resp.NewOkSimpleString()
	}

	return resp.NewError("ERR DB index is out of range")
}
