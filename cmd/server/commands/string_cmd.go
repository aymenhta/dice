package commands

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
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

func ttl_value_option(opt string) dsa.Option[SetTtlKind] {
	switch opt {
	case "ex":
		return dsa.Some(SET_TTL_EX)
	case "px":
		return dsa.Some(SET_TTL_PX)
	case "exat":
		return dsa.Some(SET_TTL_EXAT)
	case "pxat":
		return dsa.Some(SET_TTL_PXAT)
	default:
		return dsa.None[SetTtlKind]()
	}
}

type SetTtlAction uint8

const (
	SET_TTL_CLEAR SetTtlAction = iota
	SET_TTL_KEEP
	SET_TTL_AT
)

type SetCondition struct {
	cmp_value dsa.Option[[]byte]
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
		return sc.cmp_value.IsSomeAnd(func(v []byte) bool { return bytes.Compare(current, v) == 0 })
		// return bytes.Compare(current, sc.cmp_value.OrEmpty()) == 0
	case sc.kind == SET_IFEQ && current == nil:
		return false

	case sc.kind == SET_IFNE && current != nil:
		return sc.cmp_value.IsSomeAnd(func(v []byte) bool { return bytes.Compare(current, v) != 0 })
		// return bytes.Compare(current, sc.cmp_value.OrEmpty()) != 0
	case sc.kind == SET_IFNE && current == nil:
		return true

	case sc.kind == SET_IFDEQ && current != nil:
		return sc.cmp_value.IsSomeAnd(func(v []byte) bool { return utils.BytesToXXH3hex(current) == utils.BStr(v) })
		// return utils.BytesToXXH3hex(current) == utils.BStr(sc.cmp_value.OrEmpty())
	case sc.kind == SET_IFDEQ && current == nil:
		return false

	case sc.kind == SET_IFDNE && current != nil:
		return sc.cmp_value.IsSomeAnd(func(v []byte) bool { return utils.BytesToXXH3hex(current) != utils.BStr(v) })
		// return utils.BytesToXXH3hex(current) != utils.BStr(sc.cmp_value.OrEmpty())
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
	new_ttl dsa.Option[time.Time]
	kind    dsa.Option[SetTtlKind]
	action  SetTtlAction
}

func (kind SetTtlKind) convertSecsToMillis(n int64, cmd string) (int64, error) {
	if kind == SET_TTL_EX || kind == SET_TTL_EXAT {
		n = n * 1000
		return n, nil // TODO: check for integer overflow
	}
	if kind == SET_TTL_PX || kind == SET_TTL_PXAT {
		return n, nil
	}

	return -1, utils.InvalidExpire(cmd)
}

func (kind SetTtlKind) calculateDeadline(ms int64, cmd string) (dsa.Option[time.Time], error) {
	switch kind {
	case SET_TTL_EX, SET_TTL_PX:
		return dsa.Some(time.Now().Add(time.Duration(ms) * time.Millisecond)), nil
	case SET_TTL_EXAT, SET_TTL_PXAT:
		return dsa.Some(utils.UnixMsToInstant(ms)), nil
	default:
		return dsa.None[time.Time](), utils.InvalidExpire(cmd)
	}
}

func (kind SetTtlKind) expiryToTime(n int64, cmd string) (dsa.Option[time.Time], error) {
	if n <= 0 {
		return dsa.None[time.Time](), utils.InvalidExpire(cmd)
	}

	// convert to ms
	millis, err := kind.convertSecsToMillis(n, cmd)
	if err != nil {
		return dsa.None[time.Time](), err
	}

	// calculate deadline
	return kind.calculateDeadline(millis, cmd)
}

func (x *SetTTL) expires_at() dsa.Option[time.Time] {
	if x.action == SET_TTL_AT {
		return x.new_ttl
	}
	return dsa.None[time.Time]()
}

func (cm *RespCommandManager) cmdSet(db *store.Database, args [][]byte) error {
	if len(args) < 2 {
		return cm.WriteWrongArgsNumError("set")
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	value := utils.CopyBytes(args[1])

	cond := &SetCondition{
		cmp_value: dsa.None[[]byte](),
		kind:      SET_ALWAYS,
	}
	get := false
	ttl := &SetTTL{
		new_ttl: dsa.None[time.Time](),
		kind:    dsa.None[SetTtlKind](),
		action:  SET_TTL_CLEAR,
	}

	// parse options
	i := 2
	for i < len(args) {
		option := utils.BStr(utils.BytesToLower(args[i]))
		switch option {
		case "xx":
			if cond.kind != SET_ALWAYS && cond.kind != SET_XX {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_XX
			i = i + 1
		case "nx":
			if cond.kind != SET_ALWAYS && cond.kind != SET_NX {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_NX
			i = i + 1
		case "ifeq":
			if cond.kind != SET_ALWAYS {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_IFEQ
			cond.cmp_value = dsa.Some(utils.CopyBytes(args[i+1]))
			i = i + 2
		case "ifne":
			if cond.kind != SET_ALWAYS {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_IFNE
			cond.cmp_value = dsa.Some(utils.CopyBytes(args[i+1]))
			i = i + 2
		case "ifdeq":
			if cond.kind != SET_ALWAYS {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_IFDEQ
			cond.cmp_value = dsa.Some(utils.CopyBytes(args[i+1]))
			i = i + 2
		case "ifdne":
			if cond.kind != SET_ALWAYS {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			cond.kind = SET_IFDNE
			cond.cmp_value = dsa.Some(utils.CopyBytes(args[i+1]))
			i = i + 2
		case "get":
			get = true
			i = i + 1
		case "keepttl":
			// if ttl.kind != nil && *ttl.kind != SET_TTL_KEEPTTL {
			// 	return cm.WriteError(utils.ErrSyntaxError)
			// }
			if ttl.kind.IsSomeAnd(func(k SetTtlKind) bool { return k != SET_TTL_KEEPTTL }) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			ttl.kind = dsa.Some(SET_TTL_KEEPTTL)
			ttl.action = SET_TTL_KEEP
			i = i + 1
		default:
			kind := ttl_value_option(option)
			if kind.IsSome() {
				if i+1 >= len(args) {
					return cm.WriteError(utils.ErrSyntaxError)
				}
				// if ttl.kind != nil && *ttl.kind != *kind {
				// 	return cm.WriteError(utils.ErrSyntaxError)
				// }
				kindVal := kind.OrEmpty()
				if ttl.kind.IsSomeAnd(func(k SetTtlKind) bool { return k != kindVal }) {
					return cm.WriteError(utils.ErrSyntaxError)
				}
				n, err := utils.ParseInt64(args[i+1])
				if err != nil {
					return cm.WriteError(err)
				}
				new_ttl, err := kindVal.expiryToTime( /*kindVal,*/ n, "set")
				if err != nil {
					return cm.WriteError(err)
				}

				ttl.kind = kind
				ttl.action = SET_TTL_AT
				ttl.new_ttl = new_ttl
				i = i + 2
			} else {
				return cm.WriteError(utils.ErrSyntaxError)
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
			db.Set(key, store.NewStringEntry(value, ttl.expires_at()))
			didSet = true
		}
	} else {
		exists := !entry.IsExpired(now)

		typedEntry, tok := entry.AsStr()
		if !tok {
			return cm.WriteWrongTypeError()
		}

		if exists && typedEntry != nil {
			current = typedEntry.Value
			if get {
				old = utils.CopyBytes(typedEntry.Value)
			}
		} else if exists && typedEntry == nil {
			if get || cond.needs_value() {
				return cm.WriteWrongTypeError()
			}
		}

		allowed := cond.cond_allows(current)
		if allowed {
			expiresAt := dsa.None[time.Time]()
			if ttl.action == SET_TTL_CLEAR {
				expiresAt = dsa.None[time.Time]()
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
		return cm.WriteBytes2(old)
	} else if didSet {
		return cm.WriteSimpleString("OK")
	} else {
		return cm.WriteNullString()
	}

}

func (cm *RespCommandManager) cmdGet(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("get")
	}

	key := utils.BStr(args[0])

	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteNullString()
	}

	typedEntry, tok := entry.AsStr()
	if !tok {
		return cm.WriteWrongTypeError()
	}
	return cm.WriteBytes(typedEntry.Value)
}

func (cm *RespCommandManager) incr_decrByDelta(db *store.Database, args [][]byte, delta int64) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	entry, ok := db.Get(key)
	if !ok || entry.IsExpired(time.Now()) {
		db.Set(key, store.NewStringEntry(utils.Int64ToBytes(delta), dsa.None[time.Time]()))
		return cm.WriteInt64(delta)
	}

	typedEntry, ok := entry.AsStr()
	if !ok {
		return cm.WriteWrongTypeError()
	}

	current, err := utils.ParseInt64(typedEntry.Value)
	if err != nil {
		return cm.WriteError(err)
	}

	next, ok := utils.Add64(current, delta)
	if !ok {
		return cm.WriteError(utils.ErrIncrDecrOverflow)
	}

	typedEntry.Value = utils.Int64ToBytes(next)
	return cm.WriteInt64(next)
}

func (cm *RespCommandManager) cmdIncr(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("incr")
	}

	return cm.incr_decrByDelta(db, args, 1)
}

func (cm *RespCommandManager) cmdDecr(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("decr")
	}

	return cm.incr_decrByDelta(db, args, -1)
}

func (cm *RespCommandManager) cmdIncrBy(db *store.Database, args [][]byte) error {
	if len(args) != 2 {
		return cm.WriteWrongArgsNumError("incrby")
	}

	delta, err := utils.ParseInt64(args[1])
	if err != nil {
		return cm.WriteError(err)
	}

	return cm.incr_decrByDelta(db, args, delta)
}
func (cm *RespCommandManager) cmdDecrBy(db *store.Database, args [][]byte) error {
	if len(args) != 2 {
		return cm.WriteWrongArgsNumError("decrby")
	}

	delta, err := utils.ParseInt64(args[1])
	if err != nil {
		return cm.WriteError(err)
	}

	return cm.incr_decrByDelta(db, args, -delta)
}

func (cm *RespCommandManager) cmdMGet(db *store.Database, args [][]byte) error {
	if len(args) == 0 {
		return cm.WriteWrongArgsNumError("mget")
	}

	cm.WriteArrayHeader(len(args))
	for _, arg := range args {
		if cm.Werr() != nil {
			return cm.Werr()
		}
		key := utils.BStr(arg)
		entry, ok := lives(db, key)
		if !ok {
			cm.WriteNullString()
			continue
		}

		typedEntry, valid := entry.AsStr()
		if !valid {
			cm.WriteNullString()
		} else {
			cm.WriteBytes(typedEntry.Value)
		}
	}

	return cm.Werr()
}

// Each pair is written atomically, but the batch as a whole is NOT atomic:
// another connection can observe some keys set and others not yet.
// Like SET, MSET discards any existing TTL.
func (cm *RespCommandManager) cmdMSet(db *store.Database, args [][]byte) error {
	if len(args) != 0 && len(args)%2 != 0 {
		return cm.WriteWrongArgsNumError("mset")
	}

	pairs := make(map[string]*store.Entry)

	for pair := range slices.Chunk(args, 2) {
		key := utils.BStr(utils.CopyBytes(pair[0]))
		value := utils.CopyBytes(pair[1])
		pairs[key] = store.NewStringEntry(value, dsa.None[time.Time]())
	}

	db.MSet(pairs)
	return cm.WriteSimpleString("OK")
}

func (cm *RespCommandManager) cmdGetDel(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("getdel")
	}

	key := utils.BStr(args[0])
	value, exists, strTyped := db.GetAndRemove(key)
	if !exists || !strTyped {
		return cm.WriteNullString()
	}

	return cm.WriteBytes(value)
}

// DELEX key [IFEQ v | IFNE v | IFDEQ d | IFDNE d] -> 1 if deleted, else 0.
func (cm *RespCommandManager) cmdDelEx(db *store.Database, args [][]byte) error {
	if len(args) == 0 {
		return cm.WriteWrongArgsNumError("delex")
	}

	key := utils.BStr(args[0])
	condition := &SetCondition{
		cmp_value: dsa.None[[]byte](),
		kind:      SET_ALWAYS,
	}

	if len(args) == 1 {
		condition.kind = SET_ALWAYS
	} else if len(args) == 3 {
		option := utils.BStr(utils.BytesToLower(args[1]))
		operand := args[2]

		if option != "ifeq" && option != "ifne" && option != "ifdeq" && option != "ifdne" {
			return cm.WriteError(utils.ErrSyntaxError)
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
			return cm.WriteError(utils.ErrSyntaxError)
		}
		condition.cmp_value = dsa.Some(operand)
	} else {
		return cm.WriteError(utils.ErrSyntaxError)
	}

	now := time.Now()
	entry, ok := db.Get(key)
	if !ok {
		return cm.WriteInt64(0)
	}

	if entry.IsExpired(now) {
		db.Remove(key) // lazily evict
		return cm.WriteInt64(0)
	}

	if condition.needs_value() {
		strEntry, tok := entry.AsStr()
		if !tok { // only string values are allowed to be compared
			return cm.WriteError(errors.New("Key should be of string type if conditions are specified"))
		}

		allowed := condition.cond_allows(strEntry.Value)
		if !allowed {
			return cm.WriteInt64(0)
		}
	}

	db.Remove(key)
	return cm.WriteInt64(1)
}

// GETEX key [EX s | PX ms | EXAT ts | PXAT ms-ts | PERSIST]
func (cm *RespCommandManager) cmdGetEx(db *store.Database, args [][]byte) error {
	if len(args) == 0 {
		return cm.WriteWrongArgsNumError("getex")
	}

	key := utils.BStr(args[0])
	ttl := &SetTTL{action: SET_TTL_KEEP}

	i := 1
	for i < len(args) {
		opt := utils.BStr(utils.BytesToLower(args[i]))
		if opt == "persist" {
			// if ttl.kind != nil && *ttl.kind != SET_TTL_PERSIST {
			// 	return cm.WriteError(utils.ErrSyntaxError)
			// }
			if ttl.kind.IsSomeAnd(func(k SetTtlKind) bool { return k != SET_TTL_PERSIST }) {
				return cm.WriteError(utils.ErrSyntaxError)
			}
			ttl.kind = dsa.Some(SET_TTL_PERSIST)
			ttl.action = SET_TTL_CLEAR
			i += 1
		} else {
			k := ttl_value_option(opt)
			if k.IsSome() {
				if i+1 >= len(args) {
					return cm.WriteError(utils.ErrSyntaxError)
				}

				kVal := k.OrEmpty()

				if ttl.kind.IsSomeAnd(func(k SetTtlKind) bool { return k != kVal }) {
					return cm.WriteError(utils.ErrSyntaxError)
				}

				n, err := utils.ParseInt64(args[i+1])
				if err != nil {
					return cm.WriteError(err)
				}

				t, err := kVal.expiryToTime( /*kVal,*/ n, "getex")
				if err != nil {
					return cm.WriteError(err)
				}

				ttl.new_ttl = t
				ttl.action = SET_TTL_AT
				ttl.kind = k
				i += 2
			} else {
				return cm.WriteError(utils.ErrSyntaxError)
			}
		}
	}

	now := time.Now()
	entry, ok := db.Get(key)
	if !ok {
		return cm.WriteNullString()
	}

	if entry.IsExpired(now) {
		db.Remove(key)
		return cm.WriteNullString()
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	switch ttl.action {
	case SET_TTL_CLEAR:
		entry.Ttl = dsa.None[time.Time]()
	case SET_TTL_AT:
		entry.Ttl = ttl.new_ttl
	}

	return cm.WriteBytes(strEntry.Value)
}

func (cm *RespCommandManager) cmdStrLen(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("strlen")
	}

	key := utils.BStr(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteInt64(0)
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	return cm.WriteInt(len(strEntry.Value))
}

func (cm *RespCommandManager) cmdAppend(db *store.Database, args [][]byte) error {
	if len(args) != 2 {
		return cm.WriteWrongArgsNumError("append")
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	suffix := utils.CopyBytes(args[1])

	now := time.Now()

	entry, ok := db.Get(key)
	if !ok || entry.IsExpired(now) {
		l := len(suffix)
		db.Set(key, store.NewStringEntry(suffix, dsa.None[time.Time]()))
		return cm.WriteInt(l)
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	nl := len(strEntry.Value) + len(suffix)
	if nl > cm.MaxBulkLength() {
		return cm.WriteError(utils.ErrProtoInvalidBulkLength)
	}

	var buff bytes.Buffer
	buff.Write(strEntry.Value)
	buff.Write(suffix)
	strEntry.Value = buff.Bytes()

	return cm.WriteInt(nl)
}

func (cm *RespCommandManager) cmdDigest(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteWrongArgsNumError("digest")
	}

	key := utils.BStr(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteExplicitNullString()
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return cm.WriteWrongTypeError()
	}

	digest := utils.BytesToXXH3hex(strEntry.Value)
	return cm.WriteString(digest)
}

func (cm *RespCommandManager) cmdGetRange(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	start, err := utils.ParseInt64(args[1])
	if err != nil {
		return cm.WriteError(err)
	}
	end, err := utils.ParseInt64(args[2])
	if err != nil {
		return cm.WriteError(err)
	}

	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteString("")
	}

	strEntry, ok := entry.AsStr()
	if !ok {
		return cm.WriteWrongTypeError()
	}

	s, e, ok := getrangeBounds(len(strEntry.Value), start, end)
	if !ok {
		return cm.WriteString("")
	}

	return cm.WriteBytes(strEntry.Value[s : e+1])
}

func (cm *RespCommandManager) cmdSetRange(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	offset, err := utils.ParseInt64(args[1])
	if err != nil {
		return cm.WriteError(err)
	}
	if offset < 0 {
		return cm.WriteError(fmt.Errorf("offset is out of range"))
	}
	value := args[2]
	// an empty value is a no-op that reports the current
	// length, and never creates the key.
	if len(value) == 0 {
		entry, ok := lives(db, key)
		if !ok {
			return cm.WriteInt64(0)
		}

		strE, ok := entry.AsStr()
		if !ok {
			return cm.WriteWrongTypeError()
		}

		return cm.WriteInt(len(strE.Value))
	}

	needed, ok := utils.Add64(offset, int64(len(value)))
	if !ok || needed > int64(cm.MaxBulkLength()) {
		return cm.WriteError(utils.ErrProtoInvalidBulkLength)
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
		db.Set(key, store.NewStringEntry(fresh(), dsa.None[time.Time]()))
		return cm.WriteInt64(needed)
	}

	strE, ok := entry.AsStr()
	if !ok {
		return cm.WriteWrongTypeError()
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
	return cm.WriteInt(l)
}

type MSetExArgs struct {
	nKeys          int64
	fields         []dsa.Tuple[string, []byte]
	cond           dsa.Option[string]
	expirationKind dsa.Option[string]
	expiration     dsa.Option[time.Time]
}

func mSetExConvertSecsToMillis(kind string, n int64) (int64, error) {
	if kind == "ex" || kind == "exat" {
		n = n * 1000
		return n, nil // TODO: check for integer overflow
	}
	if kind == "px" || kind == "pxat" {
		return n, nil
	}

	return -1, utils.InvalidExpire("msetex")
}

func mSetExCalculateDeadline(kind string, ms int64) (dsa.Option[time.Time], error) {
	switch kind {
	case "ex", "px":
		return dsa.Some(time.Now().Add(time.Duration(ms) * time.Millisecond)), nil
	case "exat", "pxat":
		return dsa.Some(utils.UnixMsToInstant(ms)), nil
	default:
		return dsa.None[time.Time](), utils.InvalidExpire("msetex")
	}
}

func mSetExExpiryToTime(kind string, n int64) (dsa.Option[time.Time], error) {
	if n <= 0 {
		return dsa.None[time.Time](), utils.InvalidExpire("msetex")
	}

	// convert to ms
	millis, err := mSetExConvertSecsToMillis(kind, n)
	if err != nil {
		return dsa.None[time.Time](), err
	}

	// calculate deadline
	return mSetExCalculateDeadline(kind, millis)
}

func parseMSetExArgs(args [][]byte) (*MSetExArgs, error) {
	nkeys, err := utils.ParseInt64(args[0])
	if err != nil {
		return nil, err
	}
	if nkeys < 1 {
		return nil, utils.ErrSyntaxError
	}

	offset := nkeys*2 + 1
	rawFields := args[1:offset]
	if len(rawFields)%2 != 0 {
		return nil, utils.ErrSyntaxError
	}

	fields := make([]dsa.Tuple[string, []byte], 0, nkeys)
	for pair := range slices.Chunk(rawFields, 2) {
		fields = append(fields, dsa.NewTuple(
			utils.BStr(utils.CopyBytes(pair[0])),
			utils.CopyBytes(pair[1]),
		))
	}

	cond := dsa.None[string]()
	expKind := dsa.None[string]()
	exp := dsa.None[time.Time]()

	i := int(offset)
	for i < len(args) {
		option := utils.BStr(utils.BytesToLower(args[i]))
		switch option {
		case "nx":
			if cond.IsSomeAnd(func(v string) bool { return v != "nx" }) {
				return nil, utils.ErrSyntaxError
			}
			cond = dsa.Some("nx")
			i += 1
		case "xx":
			if cond.IsSomeAnd(func(v string) bool { return v != "xx" }) {
				return nil, utils.ErrSyntaxError
			}
			cond = dsa.Some("xx")
			i += 1
		case "keepttl":
			if expKind.IsSomeAnd(func(v string) bool { return v != "keepttl" }) {
				return nil, utils.ErrSyntaxError
			}
			expKind = dsa.Some("keepttl")
			i = i + 1
		default:
			if option == "ex" || option == "px" || option == "exat" || option == "pxat" {
				if i+1 >= len(args) {
					return nil, utils.ErrSyntaxError
				}
				if expKind.IsSomeAnd(func(k string) bool { return k != option }) {
					return nil, utils.ErrSyntaxError
				}
				n, err := utils.ParseInt64(args[i+1])
				if err != nil {
					return nil, err
				}
				new_ttl, err := mSetExExpiryToTime(option, n)
				if err != nil {
					return nil, err
				}

				expKind = dsa.Some(option)
				exp = new_ttl
				i = i + 2
			} else {
				return nil, utils.ErrSyntaxError
			}
		}
	}

	return &MSetExArgs{
		nKeys:          nkeys,
		fields:         fields,
		cond:           cond,
		expirationKind: expKind,
		expiration:     exp,
	}, nil
}

func (mc *RespCommandManager) cmdMSetEx(db *store.Database, args [][]byte) error {
	opts, err := parseMSetExArgs(args)
	if err != nil {
		return mc.WriteError(err)
	}

	n, err := db.MSetEx(
		opts.nKeys,
		opts.fields,
		opts.cond,
		opts.expirationKind,
		opts.expiration,
	)
	if err != nil {
		return mc.WriteError(err)
	}

	return mc.WriteInt64(n)
}

func (mc *RespCommandManager) cmdMSetNx(db *store.Database, args [][]byte) error {
	pairs := make(map[string]*store.Entry)
	for a := range slices.Chunk(args, 2) {
		k := utils.BStr(utils.CopyBytes(a[0]))
		v := utils.CopyBytes(a[1])
		pairs[k] = store.NewStringEntry(v, dsa.None[time.Time]())
	}

	// all keys must not exist
	for k := range pairs {
		if _, ok := lives(db, k); ok {
			return mc.WriteInt64(0)
		}
	}

	db.MSet(pairs)
	return mc.WriteInt64(1)
}

func (mc *RespCommandManager) cmdSetNx(db *store.Database, args [][]byte) error {
	k := utils.BStr(utils.CopyBytes(args[0]))
	if _, ok := lives(db, k); ok {
		return mc.WriteInt64(0)
	}

	v := store.NewStringEntry(utils.CopyBytes(args[1]), dsa.None[time.Time]())
	db.Set(k, v)
	return mc.WriteInt64(1)
}
