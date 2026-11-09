package store

import (
	"time"

	"aymenhta.com/dice/internal/utils/mo"
)

type EntryKind uint8

const (
	STRING_KIND EntryKind = iota
	HASH_KIND
)

type IEntryType interface {
	Kind() EntryKind
	TypeName() string
}

type String struct {
	Value []byte
}

func (s *String) Kind() EntryKind {
	return STRING_KIND
}

func (s *String) TypeName() string {
	return "string"
}

type HashField struct {
	Value []byte
	Ttl   mo.Option[time.Time]
}

func NewHashField(value []byte) *HashField {
	return &HashField{Value: value, Ttl: mo.None[time.Time]()}
}

func NewHashFieldWithTTL(value []byte, ttl mo.Option[time.Time]) *HashField {
	return &HashField{Value: value, Ttl: ttl}
}

func (e *HashField) IsExpired(now time.Time) bool {
	if e.Ttl.IsNone() {
		return false
	}

	return now.Equal(e.Ttl.OrEmpty()) || now.After(e.Ttl.OrEmpty())
}

type Hash struct {
	Value map[string]*HashField
}

func (h *Hash) Kind() EntryKind {
	return HASH_KIND
}

func (s *Hash) TypeName() string {
	return "hash"
}
