package store

import "time"

type EntryKind uint8

const (
	STRING_KIND EntryKind = iota
	HASH_KIND
)

type IEntryType interface {
	Kind() EntryKind
}

type String struct {
	Value []byte
}

func (s *String) Kind() EntryKind {
	return STRING_KIND
}

type HashField struct {
	Value []byte
	Ttl   *time.Duration
}

type Hash struct {
	Value map[string]*HashField
}

func (h *Hash) Kind() EntryKind {
	return HASH_KIND
}
