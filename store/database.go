package store

import (
	"fmt"
	"sync"
	"time"
)

type Database[K comparable] struct {
	sync.RWMutex
	body map[K]*Entry
}

func NewDB[V any]() *Database[string] {
	return &Database[string]{
		body: make(map[string]*Entry),
	}
}

func (db *Database[K]) Set(key K, value *Entry) {
	db.Lock()
	db.body[key] = value
	db.Unlock()
}

func (db *Database[K]) Get(key K) (*Entry, bool) {
	db.RLock()
	v, ok := db.body[key]
	db.RUnlock()
	return v, ok
}

func (db *Database[K]) Remove(key K) {
	db.Lock()
	delete(db.body, key)
	db.Unlock()
}

func (db *Database[K]) BatchRemove(keys []K) int64 {
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

func (db *Database[K]) MSet(pairs map[K]*Entry) {
	db.Lock()
	for k, v := range pairs {
		db.body[k] = v
	}
	db.Unlock()
}

func (db *Database[K]) GetAndRemove(key K) ([]byte, bool, bool) {
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

func (db *Database[K]) Size() int {
	db.RLock()
	defer db.RUnlock()
	return len(db.body)
}

func (db *Database[K]) Clear() {
	db.Lock()
	clear(db.body)
	defer db.Unlock()
}

func (db *Database[K]) debugMap() {
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
