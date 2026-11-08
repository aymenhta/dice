package commands

import (
	"errors"
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
func (cm *RespCommandManager) cmdHSet(db *store.Database, args [][]byte) error {
	if !(len(args) >= 3 && len(args)%2 == 1) {
		return cm.WriteError(utils.WrongArgsNum("hset"))
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
		return cm.WriteInt64(added)
	}

	hashEntry, tok := entry.AsHash()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	added := 0
	for _, pair := range pairs {
		_, ok := hashEntry.Value[pair.field]
		hashEntry.Value[pair.field] = store.NewHashField(pair.value)
		if !ok {
			added += 1
		}
	}
	return cm.WriteInt(added)
}

// HGET key field -> the field's value, or nil if the key or field is missing
func (cm *RespCommandManager) cmdHGet(db *store.Database, args [][]byte) error {
	if len(args) != 2 {
		return cm.WriteError(utils.WrongArgsNum("hget"))
	}

	key := utils.BStr(args[0])
	field := utils.BStr(args[1])
	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteNullString()
	}

	hashEntry, tok := entry.AsHash()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	hashField, fok := hashEntry.Value[field]
	if !fok {
		return cm.WriteNullString()
	}

	return cm.WriteBytes(hashField.Value)
}

// HGETALL key -> a flat array of field, value, field, value, ...
// Empty for a missing key. Field order is unspecified.
func (cm *RespCommandManager) cmdHGetAll(db *store.Database, args [][]byte) error {
	entry, ok := lives(db, utils.BStr(args[0]))
	if !ok {
		return cm.WriteArray(nil)
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return cm.WriteWrongTypeError()
	}

	out := make([][]byte, 0, len(hashE.Value)*2)
	for field, value := range hashE.Value {
		out = append(out, utils.StrB(field))
		out = append(out, value.Value)
	}
	return cm.WriteArrayBytes(out)
}

// HDEL key field [field ...] -> how many of those fields were removed.
func (mc *RespCommandManager) cmdHDel(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	fields := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		fields = append(fields, utils.BStr(arg))
	}

	removed, err := db.RemoveHashFields(key, fields, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt(removed)
}

func (mc *RespCommandManager) cmdHExists(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	field := utils.BStr(args[1])

	entry, ok := lives(db, key)
	if !ok {
		return mc.WriteInt64(0)
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	if _, ok := hashE.Value[field]; !ok {
		return mc.WriteInt64(0)
	}
	return mc.WriteInt64(1)
}

func (mc *RespCommandManager) cmdHLen(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])

	entry, ok := lives(db, key)
	if !ok {
		return mc.WriteInt64(0)
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	return mc.WriteInt(len(hashE.Value))
}

// HMGET key field [field ...] -> one entry per requested field, in the
// requested order (a repeated field is repeated), nil where missing.
func (mc *RespCommandManager) cmdHMGet(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	fields := make([]string, 0, len(args)-1)
	for _, arg := range args[1:] {
		fields = append(fields, utils.BStr(utils.CopyBytes(arg)))
	}

	entry, ok := lives(db, key)
	if !ok {
		mc.WriteArrayHeader(len(fields))
		for i := 0; i < len(fields); i++ {
			mc.WriteNullString()
		}

		return mc.Werr()
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	mc.WriteArrayHeader(len(fields))
	for _, f := range fields {
		hf, ok := hashE.Value[f]
		if !ok {
			mc.WriteNullString()
		} else {
			mc.WriteBytes(hf.Value)
		}
	}

	return mc.Werr()
}

func (mc *RespCommandManager) cmdHKeysOrVals(db *store.Database, args [][]byte, keys bool) error {
	entry, ok := lives(db, utils.BStr(args[0]))
	if !ok {
		return mc.WriteArray([]any{})
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	mc.WriteArrayHeader(len(hashE.Value))
	if keys {
		for k, _ := range hashE.Value {
			mc.WriteString(k)
		}
	} else {
		for _, v := range hashE.Value {
			mc.WriteBytes(v.Value)
		}
	}

	return mc.Werr()
}

func (mc *RespCommandManager) cmdHIncrBy(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	field := utils.BStr(utils.CopyBytes(args[1]))
	delta, err := utils.ParseInt64(args[2])
	if err != nil {
		return mc.WriteError(err)
	}
	n, err := db.IncrByHashFields(key, field, delta, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt64(n)
}
