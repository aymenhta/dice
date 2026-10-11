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

func set(db HashMap, key, value []byte) resp.Frame {
	db[string(key)] = &Entry{
		value: value,
		// expiresAt: , // TODO: implement the expires at option
	}
	return resp.NewOkSimpleString()
}

func get(db HashMap, key []byte) resp.Frame {
	entry, ok := db[string(key)]
	if !ok {
		return resp.NewNullBulkString()
	}

	return resp.NewBulkString(entry.value)
}

func del(db HashMap, keys [][]byte) resp.Frame {
	var count int64
	for _, key := range keys {
		if lives(db, key) {
			delete(db, string(key))
			count = count + 1
		}
	}

	return &resp.Integer{Value: strconv.AppendInt(nil, count, 10)}
}

func exists(db HashMap, keys [][]byte) resp.Frame {
	var count int64
	for _, key := range keys {
		if lives(db, key) {
			count = count + 1
		}
	}

	return &resp.Integer{Value: strconv.AppendInt(nil, count, 10)}
}

func incr(db HashMap, key []byte, delta int64) resp.Frame {
	var current int64
	entry, ok := db[string(key)]

	if ok {
		n, err := strconv.ParseInt(string(entry.value), 10, 64)
		if err != nil {
			return resp.NewError("ERR value is not an integer or out of range")
		}

		current = n
	}

	next := strconv.AppendInt(nil, current+delta, 10)
	db[string(key)] = &Entry{value: next}
	return &resp.Integer{Value: next}
}

func dbSize(db HashMap) resp.Frame {
	return &resp.Integer{Value: []byte(strconv.Itoa(len(db)))}
}

func flushDB(db HashMap) resp.Frame {
	for key := range db {
		delete(db, key)
	}

	return resp.NewOkSimpleString()
}

func flushAll(store Store) resp.Frame {
	for _, db := range store {
		for key := range db.hashmap {
			delete(db.hashmap, key)
		}
	}

	return resp.NewOkSimpleString()
}

func selectCMD(index []byte, currentDB *int) resp.Frame {
	n, err := strconv.Atoi(string(index))
	if err == nil && n >= 0 && n < MAX_DB_NUM {
		*currentDB = n
		return resp.NewOkSimpleString()
	}

	return resp.NewError("ERR DB index is out of range")
}
