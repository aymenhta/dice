package store

import (
	"fmt"
	"sync"
	"time"

	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/utils"
)

type Database struct {
	sync.RWMutex
	Body map[string]*Entry
}

func NewDB[V any]() *Database {
	return &Database{
		Body: make(map[string]*Entry),
	}
}

func (db *Database) Set(key string, value *Entry) {
	db.Lock()
	db.Body[key] = value
	db.Unlock()
}

func (db *Database) Get(key string) (*Entry, bool) {
	db.RLock()
	v, ok := db.Body[key]
	db.RUnlock()
	return v, ok
}

func (db *Database) Remove(key string) {
	db.Lock()
	delete(db.Body, key)
	db.Unlock()
}

func (db *Database) RemoveHashFields(key string, fields []string, now time.Time) (int, error) {
	db.Lock()
	defer db.Unlock()
	removed := 0

	entry, ok := db.Body[key]
	if !ok {
		return 0, nil
	}

	if entry.IsExpired(now) {
		delete(db.Body, key)
		return 0, nil
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return 0, utils.ErrWrongTypeErr
	}

	for _, field := range fields {
		_, ok := hashE.Value[field]
		if ok {
			delete(hashE.Value, field)
			removed += 1
		}
	}

	if len(hashE.Value) == 0 {
		delete(db.Body, key)
	}

	return removed, nil
}

func (db *Database) IncrByHashFields(key string, field string, delta int64, now time.Time) (int64, error) {
	db.Lock()
	defer db.Unlock()

	entry, ok := db.Body[key]
	if !ok || entry.IsExpired(now) {
		h := make(map[string]*HashField)
		h[field] = NewHashField(utils.Int64ToBytes(delta))
		db.Body[key] = NewHashEntry(h, dsa.None[time.Time]())
		return delta, nil
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}

	var current int64
	hf, ok := hashE.Value[field]
	if !ok || hf.IsExpired(now) {
		current = 0
	} else {
		hfv, err := utils.ParseInt64(hf.Value)
		if err != nil {
			return -1, fmt.Errorf("hash value is not an integer")
		}
		current = hfv
	}

	next, ok := utils.Add64(current, delta)
	if !ok {
		return -1, fmt.Errorf("increment or decrement would overflow")
	}

	hf.Value = utils.Int64ToBytes(next)
	return next, nil
}

func (db *Database) BatchRemove(keys []string) int64 {
	removed := int64(0)
	db.Lock()
	for _, key := range keys {
		_, ok := db.Body[key]
		if ok {
			delete(db.Body, key)
			removed += 1
		}
	}
	db.Unlock()
	// db.debugMap()
	return removed
}

func (db *Database) MSet(pairs map[string]*Entry) {
	db.Lock()
	for k, v := range pairs {
		db.Body[k] = v
	}
	db.Unlock()
}

func (db *Database) GetAndRemove(key string) ([]byte, bool, bool) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	entry, ok := db.Body[key]
	if !ok {
		return nil, false, false
	}

	if entry.IsExpired(now) {
		delete(db.Body, key)
		return nil, false, false
	}

	typedEntry, tok := entry.AsStr()
	if !tok {
		return nil, true, false
	}

	value := copyBytes(typedEntry.Value)
	delete(db.Body, key)
	return value, true, true
}

func (db *Database) Size() int {
	db.RLock()
	defer db.RUnlock()
	return len(db.Body)
}

func (db *Database) Clear() {
	db.Lock()
	clear(db.Body)
	defer db.Unlock()
}

func (db *Database) LPush(key string, elems [][]byte) (int, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.Body[key]
	if !ok || e.IsExpired(now) {
		var dq dsa.Deque[[]byte]
		for _, el := range elems {
			dq.PushFront(el)
		}
		db.Body[key] = NewListEntry(&dq, dsa.None[time.Time]())
		return dq.Len(), nil
	}

	le, ok := e.AsList()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}
	for _, el := range elems {
		le.Value.PushFront(el)
	}

	return le.Value.Len(), nil
}

func (db *Database) RPush(key string, elems [][]byte) (int, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.Body[key]
	if !ok || e.IsExpired(now) {
		var dq dsa.Deque[[]byte]
		for _, el := range elems {
			dq.PushBack(el)
		}
		db.Body[key] = NewListEntry(&dq, dsa.None[time.Time]())
		return dq.Len(), nil
	}

	le, ok := e.AsList()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}
	for _, el := range elems {
		le.Value.PushBack(el)
	}

	return le.Value.Len(), nil
}

func (db *Database) LPop(key string, count dsa.Option[int64]) ([][]byte, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.Body[key]
	if !ok {
		return nil, utils.ErrRecordDoesntExist
	}

	if e.IsExpired(now) {
		delete(db.Body, key)
		return nil, utils.ErrRecordDoesntExist
	}

	le, ok := e.AsList()
	if !ok {
		return nil, utils.ErrWrongTypeErr
	}

	if le.Value.IsEmpty() {
		return nil, utils.ErrRecordDoesntExist
	}

	if count.IsSome() {
		c := count.OrEmpty()
		response := make([][]byte, 0, c)
		i := int64(0)
		for !le.Value.IsEmpty() && i < c {
			response = append(response, le.Value.PopFront())
			i += 1
		}
		return response, nil
	}

	return [][]byte{le.Value.PopFront()}, nil
}

func (db *Database) debugMap() {
	println("====================================================")
	fmt.Println("key -> value")
	for key, value := range db.Body {
		fmt.Printf("%s -> %+v\n", key, value)
	}
	println("====================================================")
}

func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
