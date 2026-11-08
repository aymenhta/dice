package store

import (
	"fmt"
	"sync"
	"time"

	"aymenhta.com/dice/internal/utils"
)

type Database struct {
	sync.RWMutex
	body map[string]*Entry
}

func NewDB[V any]() *Database {
	return &Database{
		body: make(map[string]*Entry),
	}
}

func (db *Database) Set(key string, value *Entry) {
	db.Lock()
	db.body[key] = value
	db.Unlock()
}

func (db *Database) Get(key string) (*Entry, bool) {
	db.RLock()
	v, ok := db.body[key]
	db.RUnlock()
	return v, ok
}

func (db *Database) Remove(key string) {
	db.Lock()
	delete(db.body, key)
	db.Unlock()
}

func (db *Database) RemoveHashFields(key string, fields []string, now time.Time) (int, error) {
	db.Lock()
	defer db.Unlock()
	removed := 0

	entry, ok := db.body[key]
	if !ok {
		return 0, nil
	}

	if entry.IsExpired(now) {
		delete(db.body, key)
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
		delete(db.body, key)
	}

	return removed, nil
}

func (db *Database) BatchRemove(keys []string) int64 {
	removed := int64(0)
	db.Lock()
	for _, key := range keys {
		_, ok := db.body[key]
		if ok {
			delete(db.body, key)
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
		db.body[k] = v
	}
	db.Unlock()
}

func (db *Database) GetAndRemove(key string) ([]byte, bool, bool) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	entry, ok := db.body[key]
	if !ok {
		return nil, false, false
	}

	if entry.IsExpired(now) {
		delete(db.body, key)
		return nil, false, false
	}

	typedEntry, tok := entry.AsStr()
	if !tok {
		return nil, true, false
	}

	value := copyBytes(typedEntry.Value)
	delete(db.body, key)
	return value, true, true
}

func (db *Database) Size() int {
	db.RLock()
	defer db.RUnlock()
	return len(db.body)
}

func (db *Database) Clear() {
	db.Lock()
	clear(db.body)
	defer db.Unlock()
}

func (db *Database) debugMap() {
	println("====================================================")
	fmt.Println("key -> value")
	for key, value := range db.body {
		fmt.Printf("%s -> %+v\n", key, value)
	}
	println("====================================================")
}

func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
