package client_handler

import (
	"slices"
	"time"

	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

/*
 * ===============================================
 * HASHES
 * ===============================================
 */

type HsetPair struct {
	field string
	value []byte
}

func makeHsetPairs(args [][]byte) []HsetPair {
	out := make([]HsetPair, 0, (len(args)-1)/2)
	for pair := range slices.Chunk(args[1:], 2) {
		out = append(out, HsetPair{field: utils.BStr(utils.CopyBytes(pair[0])), value: utils.CopyBytes(pair[1])})
	}
	return out
}

// Builds a hash from (field, value) pairs. Returns it with the number of
// *distinct* fields, which is what HSET reports: a field repeated within
// one call counts once, and its last value wins.
func buildHash(pairs []HsetPair) (map[string]*store.HashField, int64) {
	h := make(map[string]*store.HashField)
	added := int64(0)

	for _, pair := range pairs {
		_, ok := h[pair.field]
		h[pair.field] = &store.HashField{Value: pair.value, Ttl: nil}
		if !ok {
			added += 1
		}
	}

	return h, added
}

// HSET key field value [field value ...] -> the number of fields that were  ADDED
// (updating an existing field doesn't count).
func (h *ClientHandler) cmdHSet(db *store.Database[string], args [][]byte) error {
	if !(len(args) >= 3 && len(args)%2 == 1) {
		return h.WriteError(utils.WrongArgsNum("hset"))
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	pairs := makeHsetPairs(args)

	now := time.Now()
	entry, ok := db.Get(key)
	if !ok || entry.IsExpired(now) {
		hash, added := buildHash(pairs)
		db.Set(key, &store.Entry{
			Value: &store.Hash{Value: hash},
			Ttl:   nil,
		})
		return h.WriteInt64(added)
	}

	hashEntry, tok := entry.AsHash()
	if !tok {
		return h.WriteWrongTypeError()
	}

	added := 0
	for _, pair := range pairs {
		_, ok := hashEntry.Value[pair.field]
		hashEntry.Value[pair.field] = store.NewHashField(pair.value)
		if !ok {
			added += 1
		}
	}
	return h.WriteInt(added)
}

// HGET key field -> the field's value, or nil if the key or field is missing
func (h *ClientHandler) cmdHGet(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(utils.WrongArgsNum("hget"))
	}

	key := utils.BStr(args[0])
	field := utils.BStr(args[1])
	entry, ok := lives(db, key)
	if !ok {
		return h.WriteNullString()
	}

	hashEntry, tok := entry.AsHash()
	if !tok {
		return h.WriteWrongTypeError()
	}

	hashField, fok := hashEntry.Value[field]
	if !fok {
		return h.WriteNullString()
	}

	return h.WriteBytes(hashField.Value)
}
