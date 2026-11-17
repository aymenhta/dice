package commands

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
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
		h[pair.field] = store.NewHashField(pair.value)
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
		db.Set(key, store.NewHashEntry(hash, dsa.None[time.Time]()))
		return cm.WriteInt64(added)
	}

	db.Lock()
	defer db.Unlock()

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
	now := time.Now()
	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteNullString()
	}

	db.Lock()
	defer db.Unlock()
	hashEntry, tok := entry.AsHash()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	hashField, fok := hashEntry.Value[field]
	if !fok {
		return cm.WriteNullString()
	}

	if hashField.IsExpired(now) {
		delete(hashEntry.Value, field)
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

	now := time.Now()
	db.Lock()
	defer db.Unlock()
	if cm.RESP2Compat() {
		out := make([][]byte, 0, len(hashE.Value)*2)
		for field, value := range hashE.Value {
			if value.IsExpired(now) {
				delete(hashE.Value, field)
				continue
			}
			out = append(out, utils.StrB(field))
			out = append(out, value.Value)
		}
		return cm.WriteArrayBytes(out)
	}
	out := make(map[string][]byte)
	for field, value := range hashE.Value {
		if value.IsExpired(now) {
			delete(hashE.Value, field)
			continue
		}
		out[field] = value.Value
	}
	return cm.WriteMapBytes(out)
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
	now := time.Now()

	entry, ok := lives(db, key)
	if !ok {
		return mc.WriteInt64(0)
	}

	db.Lock()
	defer db.Unlock()
	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	hfield, ok := hashE.Value[field]
	if !ok {
		return mc.WriteInt64(0)
	}

	if hfield.IsExpired(now) {
		delete(hashE.Value, field)
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
	now := time.Now()
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
		} else if hf.IsExpired(now) {
			delete(hashE.Value, f)
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

type HExpireCondition uint8

const (
	HEXPIRE_ALWAYS HExpireCondition = iota
	HEXPIRE_NX
	HEXPIRE_XX
	HEXPIRE_GT
	HEXPIRE_LT
)

func hexpireCondFromBytes(s string) (HExpireCondition, error) {
	switch s {
	case "nx":
		return HEXPIRE_NX, nil
	case "xx":
		return HEXPIRE_XX, nil
	case "gt":
		return HEXPIRE_GT, nil
	case "lt":
		return HEXPIRE_LT, nil
	default:
		return 99, utils.ErrSyntaxError
	}
}

func isHExpireAllowed(cond HExpireCondition, secs time.Time, old dsa.Option[time.Time]) bool {
	switch cond {
	case HEXPIRE_ALWAYS:
		return true
	case HEXPIRE_NX:
		return old.IsNone()
	case HEXPIRE_XX:
		return old.IsSome()
	case HEXPIRE_GT:
		// v, ok := old.Get()
		// if !ok {
		// 	return false
		// }
		// return v.Before(secs)
		return old.IsSomeAnd(func(value time.Time) bool { return value.Before(secs) })
	case HEXPIRE_LT:
		// v, ok := old.Get()
		// if !ok {
		// 	return true
		// }
		// return v.After(secs)
		return old.IsNoneOr(func(v time.Time) bool { return v.After(secs) })
	default:
		return true
	}
}

type HExpireArgs struct {
	key       string
	seconds   uint64
	condition HExpireCondition
	numFields int
	fields    []string
}

func parseHExpireArgs(args [][]byte) (*HExpireArgs, error) {
	key := utils.BStr(args[0])
	secs, err := utils.ParseUInt64(args[1])
	if err != nil {
		return nil, err
	}

	var fields []string

	x := utils.BStr(utils.BytesToLower(args[2]))

	if x == "fields" && len(args) >= 5 {
		// parse without conditions
		nf, err := utils.ParseInt32(args[3])
		if err != nil {
			return nil, err
		}
		for _, f := range args[4:] {
			fields = append(fields, utils.BStr(f))
		}
		return &HExpireArgs{
			key:       key,
			seconds:   secs,
			condition: HEXPIRE_ALWAYS,
			numFields: int(nf),
			fields:    fields,
		}, nil
	}

	cond, err := hexpireCondFromBytes(x)
	if err != nil {
		return nil, err
	}
	if utils.BStr(utils.BytesToLower(args[3])) != "fields" {
		return nil, fmt.Errorf("ERR unknown argument: %s", utils.BStr(args[2]))
	}
	nf, err := utils.ParseInt32(args[4])
	if err != nil {
		return nil, err
	}
	for _, f := range args[5:] {
		fields = append(fields, utils.BStr(f))
	}

	return &HExpireArgs{
		key:       key,
		seconds:   secs,
		condition: cond,
		numFields: int(nf),
		fields:    fields,
	}, nil
}

// HEXPIRE key seconds [NX | XX | GT | LT] FIELDS numfields field [field ...]
func (mc *RespCommandManager) cmdHExpire(db *store.Database, rawArgs [][]byte) error {
	args, err := parseHExpireArgs(rawArgs)
	if err != nil {
		return mc.WriteError(err)
	}

	if args.numFields < 0 {
		return mc.WriteError(fmt.Errorf("Parameter `numFields` should be greater than 0"))
	}

	if len(args.fields) != args.numFields {
		return mc.WriteError(fmt.Errorf("wrong number of arguments"))
	}

	now := time.Now()
	entry, ok := db.Get(args.key)
	if !ok {
		x := make([]int64, 0, args.numFields)
		for i := 0; i < args.numFields; i++ {
			x = append(x, -2)
		}
		return mc.WriteArrayInt64(x)
	}

	if entry.IsExpired(now) {
		return mc.WriteArrayInt64([]int64{-2})
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	newTtl := now.Add(time.Duration(args.seconds) * time.Second)
	out := make([]int64, 0, args.numFields)

	db.Lock()
	for _, field := range args.fields {
		fv, ok := hashE.Value[field]
		if !ok {
			out = append(out, -2)
			continue
		}

		if fv.IsExpired(now) {
			delete(hashE.Value, field)
			out = append(out, -2)
			continue
		}
		oldTtl := dsa.None[time.Time]()
		if fv.Ttl.IsSome() {
			oldTtl = fv.Ttl
		}
		if !isHExpireAllowed(args.condition, newTtl, oldTtl) {
			out = append(out, 0)
			continue
		}

		if args.seconds == 0 {
			out = append(out, 2)
			continue
		}

		fv.Ttl = dsa.Some(newTtl)
		out = append(out, 1)
	}
	db.Unlock()
	return mc.WriteArrayInt64(out)
}
