package store

import (
	"time"

	"aymenhta.com/dice/internal/dsa"
)

const DB_NUM = 16

type Entry struct {
	Value IEntryType
	Ttl   dsa.Option[time.Time]
}

func NewStringEntry(value []byte, ttl dsa.Option[time.Time]) *Entry {
	return &Entry{
		Value: &StringEntry{Value: value},
		Ttl:   ttl,
	}
}

func NewHashEntry(value map[string]*HashField, ttl dsa.Option[time.Time]) *Entry {
	return &Entry{
		Value: &HashEntry{Value: value},
		Ttl:   ttl,
	}
}

func NewListEntry(value *dsa.Deque[[]byte], ttl dsa.Option[time.Time]) *Entry {
	return &Entry{
		Value: &ListEntry{Value: value},
		Ttl:   ttl,
	}
}

func (e *Entry) IsExpired(now time.Time) bool {
	if e.Ttl.IsNone() {
		return false
	}

	return now.Equal(e.Ttl.OrEmpty()) || now.After(e.Ttl.OrEmpty())
}

func (e *Entry) AsStr() (*StringEntry, bool) {
	if e.Value.Kind() != STRING_KIND {
		return nil, false
	}

	sv, ok := e.Value.(*StringEntry)
	if !ok {
		return nil, false
	}

	return sv, true
}

func (e *Entry) AsHash() (*HashEntry, bool) {
	if e.Value.Kind() != HASH_KIND {
		return nil, false
	}

	hv, ok := e.Value.(*HashEntry)
	if !ok {
		return nil, false
	}

	return hv, true
}

func (e *Entry) AsList() (*ListEntry, bool) {
	if e.Value.Kind() != LIST_KIND {
		return nil, false
	}

	lv, ok := e.Value.(*ListEntry)
	if !ok {
		return nil, false
	}

	return lv, true
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
