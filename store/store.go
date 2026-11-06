package store

import (
	"time"
)

const DB_NUM = 16

type Entry struct {
	Value IEntryType
	Ttl   *time.Time
}

func NewStringEntry(value []byte, ttl *time.Time) *Entry {
	return &Entry{
		Value: &String{Value: value},
		Ttl:   ttl,
	}
}

func (e *Entry) IsExpired(now time.Time) bool {
	if e.Ttl == nil {
		return false
	}

	return now.Equal(*e.Ttl) || now.After(*e.Ttl)
}

func (e *Entry) AsStr() (*String, bool) {
	if e.Value.Kind() != STRING_KIND {
		return nil, false
	}

	stringValue, ok := e.Value.(*String)
	if !ok {
		return nil, false
	}

	return stringValue, true
}

func (e *Entry) AsHash() (*Hash, bool) {
	if e.Value.Kind() != HASH_KIND {
		return nil, false
	}

	hashValue, ok := e.Value.(*Hash)
	if !ok {
		return nil, false
	}

	return hashValue, true
}

type Store struct {
	DB [DB_NUM]*Database[string]
}

func NewStore() *Store {
	var store Store
	for i := range DB_NUM {
		store.DB[i] = NewDB[*Entry]()
	}

	return &store
}
