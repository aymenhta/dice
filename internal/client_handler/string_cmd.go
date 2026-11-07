package client_handler

import (
	"bytes"
	"fmt"
	"slices"
	"time"

	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

type SetConditionKind uint8

const (
	SET_ALWAYS SetConditionKind = iota
	SET_NX
	SET_XX
	SET_IFEQ
	SET_IFNE
	SET_IFDEQ
	SET_IFDNE
) // TODO: add digest equality as well

type SetTtlKind uint8

const (
	SET_TTL_EX SetTtlKind = iota
	SET_TTL_PX
	SET_TTL_EXAT
	SET_TTL_PXAT
	SET_TTL_KEEPTTL
	SET_TTL_PERSIST
)

func ttl_value_option(opt string) *SetTtlKind {
	if opt == "ex" {
		return utils.Pointer(SET_TTL_EX)
	} else if opt == "px" {
		return utils.Pointer(SET_TTL_PX)
	} else if opt == "exat" {
		return utils.Pointer(SET_TTL_EXAT)
	} else if opt == "pexat" {
		return utils.Pointer(SET_TTL_PXAT)
	} else {
		return nil
	}
}

type SetTtlAction uint8

const (
	SET_TTL_CLEAR SetTtlAction = iota
	SET_TTL_KEEP
	SET_TTL_AT
)

type SetCondition struct {
	cmp_value []byte
	kind      SetConditionKind
}

func (sc *SetCondition) cond_allows(current []byte) bool {
	switch {
	case sc.kind == SET_ALWAYS:
		return true

	case sc.kind == SET_NX:
		return current == nil
	case sc.kind == SET_XX:
		return current != nil

	case sc.kind == SET_IFEQ && current != nil:
		return bytes.Compare(current, sc.cmp_value) == 0
	case sc.kind == SET_IFEQ && current == nil:
		return false

	case sc.kind == SET_IFNE && current != nil:
		return bytes.Compare(current, sc.cmp_value) != 0
	case sc.kind == SET_IFNE && current == nil:
		return true

	case sc.kind == SET_IFDEQ && current != nil:
		return utils.BytesToXXH3hex(current) == utils.BStr(sc.cmp_value)
	case sc.kind == SET_IFDEQ && current == nil:
		return false

	case sc.kind == SET_IFDNE && current != nil:
		return utils.BytesToXXH3hex(current) != utils.BStr(sc.cmp_value)
	case sc.kind == SET_IFDNE && current == nil:
		return true

	default:
		return true
	}

}

func (sc *SetCondition) needs_value() bool {
	return sc.kind == SET_IFEQ || sc.kind == SET_IFNE || sc.kind == SET_IFDEQ || sc.kind == SET_IFDNE
}

type SetTTL struct {
	new_ttl *time.Time
	kind    *SetTtlKind
	action  SetTtlAction
}

func convertSecsToMillis(kind SetTtlKind, n int64, cmd string) (int64, error) {
	if kind == SET_TTL_EX || kind == SET_TTL_EXAT {
		n = n * 1000
		return n, nil // TODO: check for integer overflow
	}
	if kind == SET_TTL_PX || kind == SET_TTL_PXAT {
		return n, nil
	}

	return -1, utils.InvalidExpire(cmd)
}

func calculateDeadline(kind SetTtlKind, ms int64, cmd string) (*time.Time, error) {
	var deadline time.Time
	var err error
	if kind == SET_TTL_EX || kind == SET_TTL_PX {
		deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
	} else if kind == SET_TTL_EXAT || kind == SET_TTL_PXAT {
		deadline = utils.UnixMsToInstant(ms)
	} else {
		err = utils.InvalidExpire(cmd)
	}

	return utils.Pointer(deadline), err
}

func expiryToTime(kind SetTtlKind, n int64, cmd string) (*time.Time, error) {
	if n <= 0 {
		return nil, utils.InvalidExpire(cmd)
	}

	// convert to ms
	millis, err := convertSecsToMillis(kind, n, cmd)
	if err != nil {
		return nil, err
	}

	// calculate deadline
	return calculateDeadline(kind, millis, cmd)
}

func (x *SetTTL) expires_at() *time.Time {
	if x.action == SET_TTL_AT {
		return x.new_ttl
	}
	return nil
}

func (h *ClientHandler) cmdSet(db *store.Database[string], args [][]byte) error {
	if len(args) < 2 {
		return h.WriteError(utils.WrongArgsNum("set"))
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	value := utils.CopyBytes(args[1])

	cond := &SetCondition{
		cmp_value: nil,
		kind:      SET_ALWAYS,
	}
	get := false
	ttl := &SetTTL{
		new_ttl: nil,
		kind:    nil,
		action:  SET_TTL_CLEAR,
	}

	// parse options
	i := 2
	for i < len(args) {
		option := utils.BStr(utils.BytesToLower(args[i]))
		if option == "xx" {
			if cond.kind != SET_ALWAYS && cond.kind != SET_XX {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_XX
			i = i + 1
		} else if option == "nx" {
			if cond.kind != SET_ALWAYS && cond.kind != SET_NX {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_NX
			i = i + 1
		} else if option == "ifeq" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(resp.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_IFEQ
			cond.cmp_value = utils.CopyBytes(args[i+1])
			i = i + 2
		} else if option == "ifne" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(resp.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_IFNE
			cond.cmp_value = utils.CopyBytes(args[i+1])
			i = i + 2
		} else if option == "ifdeq" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(resp.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_IFDEQ
			cond.cmp_value = utils.CopyBytes(args[i+1])
			i = i + 2
		} else if option == "ifdne" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(resp.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(resp.ErrSyntaxError)
			}
			cond.kind = SET_IFDNE
			cond.cmp_value = utils.CopyBytes(args[i+1])
			i = i + 2
		} else if option == "get" {
			get = true
			i = i + 1
		} else if option == "keepttl" {
			if ttl.kind != nil && *ttl.kind != SET_TTL_KEEPTTL {
				return h.WriteError(resp.ErrSyntaxError)
			}
			ttl.kind = utils.Pointer(SET_TTL_KEEPTTL)
			ttl.action = SET_TTL_KEEP
			i = i + 1
		} else {
			kind := ttl_value_option(option)
			if kind != nil {
				if i+1 >= len(args) {
					return h.WriteError(resp.ErrSyntaxError)
				}
				if ttl.kind != nil && *ttl.kind != *kind {
					return h.WriteError(resp.ErrSyntaxError)
				}
				n, err := utils.ParseInt64(args[i+1])
				if err != nil {
					return h.WriteError(err)
				}
				new_ttl, err := expiryToTime(*kind, n, "set")
				if err != nil {
					return h.WriteError(err)
				}

				ttl.kind = kind
				ttl.action = SET_TTL_AT
				ttl.new_ttl = new_ttl
				i = i + 2
			} else {
				return h.WriteError(resp.ErrSyntaxError)
			}
		}
	}

	// Insertion logic

	now := time.Now()
	var old []byte = nil
	var current []byte = nil
	var didSet bool = false

	entry, ok := db.Get(key)
	if !ok {
		allowed := cond.cond_allows(nil)
		if allowed {
			expires_at := ttl.expires_at()
			db.Set(key, store.NewStringEntry(value, expires_at))
			didSet = true
		}
	} else {
		exists := !entry.IsExpired(now)

		typedEntry, tok := entry.AsStr()
		if !tok {
			return h.WriteWrongTypeError()
		}

		if exists && typedEntry != nil {
			current = typedEntry.Value
			if get {
				old = utils.CopyBytes(typedEntry.Value)
			}
		} else if exists && typedEntry == nil {
			if get || cond.needs_value() {
				return h.WriteWrongTypeError()
			}
		}

		allowed := cond.cond_allows(current)
		if allowed {
			var expiresAt *time.Time = nil
			if ttl.action == SET_TTL_CLEAR {
				expiresAt = nil
			} else if ttl.action == SET_TTL_KEEP && exists {
				expiresAt = entry.Ttl
			} else if ttl.action == SET_TTL_AT {
				expiresAt = ttl.new_ttl
			}
			db.Set(key, store.NewStringEntry(value, expiresAt))
			didSet = true
		}
	}

	// Response building
	if get {
		return h.WriteBytes(old)
	} else if didSet {
		return h.WriteSimpleString("OK")
	} else {
		return h.WriteNullString()
	}
}

func (h *ClientHandler) cmdGet(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("get"))
	}

	key := utils.BStr(args[0])

	entry, ok := lives(db, key)
	if !ok {
		return h.WriteNullString()
	}

	typedEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}
	return h.WriteBytes(typedEntry.Value)
}

func (h *ClientHandler) incr_decrByDelta(db *store.Database[string], args [][]byte, delta int64) error {

	key := utils.BStr(utils.CopyBytes(args[0]))
	entry, ok := db.Get(key)
	if !ok {
		db.Set(key, store.NewStringEntry(utils.Int64ToBytes(delta), nil))
		return h.WriteInt64(delta)
	}

	if entry.IsExpired(time.Now()) {
		// Logically absent: start from zero, no TTL.
		db.Set(key, store.NewStringEntry(utils.Int64ToBytes(delta), nil))
		return h.WriteInt64(delta)
	}

	typedEntry, ok := entry.AsStr()
	if !ok {
		return h.WriteWrongTypeError()
	}

	current, err := utils.ParseInt64(typedEntry.Value)
	if err != nil {
		return h.WriteError(err)
	}

	next, ok := utils.Add64(current, delta)
	if !ok {
		return h.WriteError(resp.ErrIncrDecrOverflow)
	}

	typedEntry.Value = utils.Int64ToBytes(next)
	return h.WriteInt64(next)
}

func (h *ClientHandler) cmdIncr(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("incr"))
	}

	return h.incr_decrByDelta(db, args, 1)
}

func (h *ClientHandler) cmdDecr(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("decr"))
	}

	return h.incr_decrByDelta(db, args, -1)
}

func (h *ClientHandler) cmdIncrBy(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(utils.WrongArgsNum("incrby"))
	}

	delta, err := utils.ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}

	return h.incr_decrByDelta(db, args, delta)
}
func (h *ClientHandler) cmdDecrBy(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(utils.WrongArgsNum("decrby"))
	}

	delta, err := utils.ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}

	return h.incr_decrByDelta(db, args, -delta)
}

func (h *ClientHandler) cmdMGet(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(utils.WrongArgsNum("mget"))
	}

	h.WriteArrayHeader(len(args))
	for _, arg := range args {
		if h.Werr() != nil {
			return h.Werr()
		}
		key := utils.BStr(arg)
		entry, ok := lives(db, key)
		if !ok {
			h.WriteNullString()
			continue
		}

		typedEntry, valid := entry.AsStr()
		if !valid {
			h.WriteNullString()
		} else {
			h.WriteBytes(typedEntry.Value)
		}
	}

	return h.Werr()
}

// Each pair is written atomically, but the batch as a whole is NOT atomic:
// another connection can observe some keys set and others not yet.
// Like SET, MSET discards any existing TTL.
func (h *ClientHandler) cmdMSet(db *store.Database[string], args [][]byte) error {
	if len(args) != 0 && len(args)%2 != 0 {
		return h.WriteError(utils.WrongArgsNum("mset"))
	}

	pairs := make(map[string]*store.Entry)

	for pair := range slices.Chunk(args, 2) {
		key := utils.BStr(utils.CopyBytes(pair[0]))
		value := utils.CopyBytes(pair[1])
		pairs[key] = store.NewStringEntry(value, nil)
	}

	db.MSet(pairs)
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdGetDel(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("getdel"))
	}

	key := utils.BStr(args[0])
	value, exists, strTyped := db.GetAndRemove(key)
	if !exists {
		return h.WriteNullString()
	}

	if !strTyped {
		return h.WriteWrongTypeError()
	}

	return h.WriteBytes(value)
}

// DELEX key [IFEQ v | IFNE v | IFDEQ d | IFDNE d] -> 1 if deleted, else 0.
func (h *ClientHandler) cmdDelEx(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(utils.WrongArgsNum("delex"))
	}

	key := utils.BStr(args[0])
	condition := &SetCondition{kind: SET_ALWAYS}

	if len(args) == 1 {
		condition.kind = SET_ALWAYS
	} else if len(args) == 3 {
		option := utils.BStr(utils.BytesToLower(args[1]))
		operand := args[2]

		if option != "ifeq" && option != "ifne" && option != "ifdeq" && option != "ifdne" {
			return h.WriteError(resp.ErrSyntaxError)
		}

		switch option {
		case "ifeq":
			condition.kind = SET_IFEQ
			break
		case "ifne":
			condition.kind = SET_IFNE
			break
		case "ifdeq":
			condition.kind = SET_IFDEQ
			break
		case "ifdne":
			condition.kind = SET_IFDNE
			break
		default:
			return h.WriteError(resp.ErrSyntaxError)
		}
		condition.cmp_value = operand
	} else {
		return h.WriteError(resp.ErrSyntaxError)
	}

	now := time.Now()
	entry, ok := db.Get(key)
	if !ok {
		return h.WriteInt64(0)
	}

	if entry.IsExpired(now) {
		db.Remove(key) // lazily evict
		return h.WriteInt64(0)
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	allowed := condition.cond_allows(strEntry.Value)
	if !allowed {
		return h.WriteInt64(0)
	}

	db.Remove(key)
	return h.WriteInt64(1)
}

// GETEX key [EX s | PX ms | EXAT ts | PXAT ms-ts | PERSIST]
func (h *ClientHandler) cmdGetEx(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(utils.WrongArgsNum("getex"))
	}

	key := utils.BStr(args[0])
	ttl := &SetTTL{action: SET_TTL_KEEP}

	i := 1
	for i < len(args) {
		opt := utils.BStr(utils.BytesToLower(args[i]))
		if opt == "persist" {
			if ttl.kind != nil && *ttl.kind != SET_TTL_PERSIST {
				return h.WriteError(resp.ErrSyntaxError)
			}
			ttl.kind = utils.Pointer(SET_TTL_PERSIST)
			ttl.action = SET_TTL_CLEAR
			i += 1
		} else {
			k := ttl_value_option(opt)
			if k != nil {
				if i+1 >= len(args) {
					return h.WriteError(resp.ErrSyntaxError)
				}

				if ttl.kind != nil && *ttl.kind != *k {
					return h.WriteError(resp.ErrSyntaxError)
				}

				n, err := utils.ParseInt64(args[i+1])
				if err != nil {
					return h.WriteError(err)
				}

				t, err := expiryToTime(*k, n, "getex")
				if err != nil {
					return h.WriteError(err)
				}

				ttl.new_ttl = t
				ttl.action = SET_TTL_AT
				ttl.kind = k
				i += 2
			} else {
				return h.WriteError(resp.ErrSyntaxError)
			}
		}
	}

	now := time.Now()
	entry, ok := db.Get(key)
	if !ok {
		return h.WriteNullString()
	}

	if entry.IsExpired(now) {
		db.Remove(key)
		return h.WriteNullString()
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	if ttl.action == SET_TTL_CLEAR {
		entry.Ttl = nil
	} else if ttl.action == SET_TTL_AT {
		entry.Ttl = ttl.new_ttl
	}

	return h.WriteBytes(strEntry.Value)
}

func (h *ClientHandler) cmdStrLen(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("strlen"))
	}

	key := utils.BStr(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return h.WriteInt64(0)
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	return h.WriteInt(len(strEntry.Value))
}

func (h *ClientHandler) cmdAppend(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(utils.WrongArgsNum("append"))
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	suffix := utils.CopyBytes(args[1])

	now := time.Now()

	entry, ok := db.Get(key)
	if !ok || entry.IsExpired(now) {
		l := len(suffix)
		db.Set(key, store.NewStringEntry(suffix, nil))
		return h.WriteInt(l)
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	nl := len(strEntry.Value) + len(suffix)
	if nl > h.MaxBulkLength() {
		return h.WriteError(resp.ErrProtoInvalidBulkLength)
	}

	var buff bytes.Buffer
	buff.Write(strEntry.Value)
	buff.Write(suffix)
	strEntry.Value = buff.Bytes()

	return h.WriteInt(nl)
}

func (h *ClientHandler) cmdDigest(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("digest"))
	}

	key := utils.BStr(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return h.WriteNullString()
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	digest := utils.BytesToXXH3hex(strEntry.Value)
	return h.WriteString(digest)
}

func (h *ClientHandler) cmdGetRange(db *store.Database[string], args [][]byte) error {
	key := utils.BStr(args[0])
	start, err := utils.ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}
	end, err := utils.ParseInt64(args[2])
	if err != nil {
		return h.WriteError(err)
	}

	entry, ok := lives(db, key)
	if !ok {
		return h.WriteString("")
	}

	strEntry, ok := entry.AsStr()
	if !ok {
		return h.WriteWrongTypeError()
	}

	s, e, ok := getrangeBounds(len(strEntry.Value), start, end)
	if !ok {
		return h.WriteString("")
	}

	return h.WriteBytes(strEntry.Value[s : e+1])
}

func (h *ClientHandler) cmdSetRange(db *store.Database[string], args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	offset, err := utils.ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}
	if offset < 0 {
		return h.WriteError(fmt.Errorf("offset is out of range"))
	}
	value := args[2]
	// an empty value is a no-op that reports the current
	// length, and never creates the key.
	if len(value) == 0 {
		entry, ok := lives(db, key)
		if !ok {
			return h.WriteInt64(0)
		}

		strE, ok := entry.AsStr()
		if !ok {
			return h.WriteWrongTypeError()
		}

		return h.WriteInt(len(strE.Value))
	}

	needed, ok := utils.Add64(offset, int64(len(value)))
	if !ok || needed > int64(h.MaxBulkLength()) {
		return h.WriteError(resp.ErrProtoInvalidBulkLength)
	}

	// Zero-padded fresh value for the "absent" cases.
	fresh := func() []byte {
		v := make([]byte, needed)
		copy(v[offset:], value)
		return v
	}

	now := time.Now()
	entry, ok := lives(db, key)
	if !ok || entry.IsExpired(now) {
		db.Set(key, store.NewStringEntry(fresh(), nil))
		return h.WriteInt64(needed)
	}

	strE, ok := entry.AsStr()
	if !ok {
		return h.WriteWrongTypeError()
	}

	// if len(strE.Value) < int(needed) {
	// 	strE.Value = utils.Expand(strE.Value, int(needed))
	// }
	v := make([]byte, len(strE.Value))
	copy(v, strE.Value)

	if int64(len(v)) < needed {
		// Grow the slice and zero-initialize the new space (like resize with 0)
		newV := make([]byte, needed)
		copy(newV, v)
		v = newV
	}

	// Copy the new value into the offset
	copy(v[offset:], value)
	l := len(v)
	strE.Value = v
	return h.WriteInt(l)
}
