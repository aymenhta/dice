package store

import (
	"time"

	"aymenhta.com/dice/internal/dsa"
)

type EntryKind uint8

const (
	STRING_KIND EntryKind = iota
	HASH_KIND
	LIST_KIND
)

type IEntryType interface {
	Kind() EntryKind
	TypeName() string
}

/**
 * ===========================================
 * 			STRINGS
 * ===========================================
 */

type StringEntry struct {
	Value []byte
}

func (s *StringEntry) Kind() EntryKind {
	return STRING_KIND
}

func (s *StringEntry) TypeName() string {
	return "string"
}

/**
 * ===========================================
 * 			HASHES
 * ===========================================
 */

type HashField struct {
	Value []byte
	Ttl   dsa.Option[time.Time]
}

func NewHashField(value []byte) *HashField {
	return &HashField{Value: value, Ttl: dsa.None[time.Time]()}
}

func NewHashFieldWithTTL(value []byte, ttl dsa.Option[time.Time]) *HashField {
	return &HashField{Value: value, Ttl: ttl}
}

func (e *HashField) IsExpired(now time.Time) bool {
	if e.Ttl.IsNone() {
		return false
	}

	return now.Equal(e.Ttl.OrEmpty()) || now.After(e.Ttl.OrEmpty())
}

type HashEntry struct {
	Value map[string]*HashField
}

func (h *HashEntry) Kind() EntryKind {
	return HASH_KIND
}

func (h *HashEntry) TypeName() string {
	return "hash"
}

/**
 * ===========================================
 * 			LISTS
 * ===========================================
 */

type ListEntry struct {
	Value *dsa.Deque[[]byte]
}

func (l *ListEntry) Kind() EntryKind {
	return LIST_KIND
}

func (l *ListEntry) TypeName() string {
	return "list"
}
