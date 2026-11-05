package store

import (
	"time"
)

const DB_NUM = 16

type Entry struct {
	Value IEntryType
	Ttl   *time.Time
}

func (e *Entry) IsExpired(now time.Time) bool {
	if e.Ttl == nil {
		return false
	}

	return now.Equal(*e.Ttl) || now.After(*e.Ttl)
}

func EntryValue[T IEntryType](entry *Entry, kind EntryKind) (T, bool) {
	var typedEntry T

	if entry.Value.Kind() != kind {
		return typedEntry, false
	}

	typedEntry, ok := entry.Value.(T)
	if !ok {
		return typedEntry, false
	}

	return typedEntry, true
}

type Store struct {
	DB [DB_NUM]*Database[string, *Entry]
}

func NewStore() *Store {
	var store Store
	for i := range DB_NUM {
		store.DB[i] = NewDB[*Entry]()
	}

	return &store
}
