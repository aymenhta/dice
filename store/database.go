package store

import (
	"fmt"
	"sync"
)

type Database[K comparable, V any] struct {
	sync.RWMutex
	body map[K]V
}

func NewDB[V any]() *Database[string, V] {
	return &Database[string, V]{
		body: make(map[string]V),
	}
}

func (db *Database[K, V]) Set(key K, value V) {
	db.Lock()
	db.body[key] = value
	db.Unlock()
}

func (db *Database[K, V]) Get(key K) (V, bool) {
	db.RLock()
	v, ok := db.body[key]
	db.RUnlock()
	return v, ok
}

func (db *Database[K, V]) Remove(key K) {
	db.Lock()
	delete(db.body, key)
	db.Unlock()
}

func (db *Database[K, V]) BatchRemove(keys []K) int64 {
	removed := int64(0)
	fmt.Printf("this is the current database before: %+v\n", db.body)
	db.Lock()
	for _, key := range keys {
		_, ok := db.body[key]
		if ok {
			delete(db.body, key)
			removed += 1
		}
	}
	db.Unlock()
	db.debugMap()
	return removed
}

func (db *Database[K, V]) debugMap() {
	println("====================================================")
	fmt.Println("key -> value")
	for key, value := range db.body {
		fmt.Printf("%s -> %+v\n", key, value)
	}
	println("====================================================")
}
