package store

import "time"

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
	Ttl   *time.Time
}

func NewHashField(value []byte) *HashField {
	return &HashField{Value: value, Ttl: nil}
}

func NewHashFieldWithTTL(value []byte, ttl *time.Time) *HashField {
	return &HashField{Value: value, Ttl: ttl}
}

func (e *HashField) IsExpired(now time.Time) bool {
	if e.Ttl == nil {
		return false
	}

	return now.Equal(*e.Ttl) || now.After(*e.Ttl)
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
