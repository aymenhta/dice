package store

import (
	"time"

	"aymenhta.com/dice/internal/utils/mo"
)

const DB_NUM = 16

type Entry struct {
	Value IEntryType
	Ttl   mo.Option[time.Time]
}

func NewStringEntry(value []byte, ttl mo.Option[time.Time]) *Entry {
	return &Entry{
		Value: &String{Value: value},
		Ttl:   ttl,
	}
}

func NewHashEntry(value map[string]*HashField, ttl mo.Option[time.Time]) *Entry {
	return &Entry{
		Value: &Hash{Value: value},
		Ttl:   ttl,
	}
}

func (e *Entry) IsExpired(now time.Time) bool {
	if e.Ttl.IsNone() {
		return false
	}

	return now.Equal(e.Ttl.OrEmpty()) || now.After(e.Ttl.OrEmpty())
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
	DB [DB_NUM]*Database
}

func NewStore() *Store {
	var store Store
	for i := range DB_NUM {
		store.DB[i] = NewDB[*Entry]()
	}

	return &store
}
