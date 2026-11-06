package resp

import (
	"bytes"
	"fmt"
	"slices"
	"time"

	"aymenhta.com/dice/store"
)

func (h *ClientHandler) cmdPing(args [][]byte) error {
	if len(args) > 1 {
		return h.WriteError(wrongArgsNum("ping"))
	}

	if len(args) == 1 {
		return h.WriteBytes(args[0])
	}

	return h.WriteSimpleString("PONG")
}

func (h *ClientHandler) cmdEcho(args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("echo"))
	}
	return h.WriteBytes(args[0])
}

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
		return Pointer(SET_TTL_EX)
	} else if opt == "px" {
		return Pointer(SET_TTL_PX)
	} else if opt == "exat" {
		return Pointer(SET_TTL_EXAT)
	} else if opt == "pexat" {
		return Pointer(SET_TTL_PXAT)
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
		return BytesToXXH3hex(current) == bstring(sc.cmp_value)
	case sc.kind == SET_IFDEQ && current == nil:
		return false

	case sc.kind == SET_IFDNE && current != nil:
		return BytesToXXH3hex(current) != bstring(sc.cmp_value)
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

	return -1, invalidExpire(cmd)
}

func calculateDeadline(kind SetTtlKind, ms int64, cmd string) (*time.Time, error) {
	var deadline time.Time
	var err error
	if kind == SET_TTL_EX || kind == SET_TTL_PX {
		deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
	} else if kind == SET_TTL_EXAT || kind == SET_TTL_PXAT {
		deadline = UnixMsToInstant(ms)
	} else {
		err = invalidExpire(cmd)
	}

	return Pointer(deadline), err
}

func expiryToTime(kind SetTtlKind, n int64, cmd string) (*time.Time, error) {
	if n <= 0 {
		return nil, invalidExpire(cmd)
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
		return h.WriteError(wrongArgsNum("set"))
	}

	key := bstring(copyBytes(args[0]))
	value := copyBytes(args[1])

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
		option := bstring(bytesToLower(args[i]))
		if option == "xx" {
			if cond.kind != SET_ALWAYS && cond.kind != SET_XX {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_XX
			i = i + 1
		} else if option == "nx" {
			if cond.kind != SET_ALWAYS && cond.kind != SET_NX {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_NX
			i = i + 1
		} else if option == "ifeq" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_IFEQ
			cond.cmp_value = copyBytes(args[i+1])
			i = i + 2
		} else if option == "ifne" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_IFNE
			cond.cmp_value = copyBytes(args[i+1])
			i = i + 2
		} else if option == "ifdeq" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_IFDEQ
			cond.cmp_value = copyBytes(args[i+1])
			i = i + 2
		} else if option == "ifdne" {
			if cond.kind != SET_ALWAYS {
				return h.WriteError(ErrSyntaxError)
			}
			if i+1 >= len(args) {
				return h.WriteError(ErrSyntaxError)
			}
			cond.kind = SET_IFDNE
			cond.cmp_value = copyBytes(args[i+1])
			i = i + 2
		} else if option == "get" {
			get = true
			i = i + 1
		} else if option == "keepttl" {
			if ttl.kind != nil && *ttl.kind != SET_TTL_KEEPTTL {
				return h.WriteError(ErrSyntaxError)
			}
			ttl.kind = Pointer(SET_TTL_KEEPTTL)
			ttl.action = SET_TTL_KEEP
			i = i + 1
		} else {
			kind := ttl_value_option(option)
			if kind != nil {
				if i+1 >= len(args) {
					return h.WriteError(ErrSyntaxError)
				}
				if ttl.kind != nil && *ttl.kind != *kind {
					return h.WriteError(ErrSyntaxError)
				}
				n, err := ParseInt64(args[i+1])
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
				return h.WriteError(ErrSyntaxError)
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
				old = copyBytes(typedEntry.Value)
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
		return h.WriteError(wrongArgsNum("get"))
	}

	key := bstring(args[0])

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

func (h *ClientHandler) cmdDel(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(wrongArgsNum("del"))
	}

	toBeDeleted := make([]string, 0)

	for i := 0; i < len(args); i++ {
		key := bstring(args[i])
		toBeDeleted = append(toBeDeleted, key)
	}

	removed := db.BatchRemove(toBeDeleted)
	return h.WriteInt64(removed)
}

func (h *ClientHandler) cmdExists(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(wrongArgsNum("exists"))
	}

	count := int64(0)
	for i := 0; i < len(args); i++ {
		key := bstring(args[i])
		if _, ok := lives(db, key); ok {
			count += 1
		}
	}

	return h.WriteInt64(count)
}

func isExpireAllowed(cond []byte, new_ttl time.Time, old_ttl *time.Time) (bool, error) {
	if cond == nil || len(cond) == 0 {
		return true, nil
	}

	switch bstring(bytesToLower(cond)) {
	case "nx":
		return old_ttl == nil, nil
	case "xx":
		return old_ttl != nil, nil
	case "gt":
		return old_ttl != nil && new_ttl.After(*old_ttl), nil
	case "lt":
		return old_ttl == nil || new_ttl.Before(*old_ttl), nil
	default:
		return false, ErrSyntaxError
	}
}

func (h *ClientHandler) cmdExpire(db *store.Database[string], args [][]byte) error {
	if len(args) < 2 || len(args) > 3 {
		return h.WriteError(wrongArgsNum("expire"))
	}

	key := bstring(args[0])
	secs, err := ParseUInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}
	if secs < 0 {
		return h.WriteError(invalidExpire("expire"))
	}

	entry, ok := db.Get(key)
	if !ok {
		return h.WriteInt(0)
	}

	now := time.Now().Add(time.Duration(secs) * time.Second)
	var condition []byte = nil
	if len(args) == 3 {
		condition = args[2]
	}

	allowed, err := isExpireAllowed(condition, now, entry.Ttl)
	if err != nil {
		return h.WriteError(err)
	}

	if !allowed {
		return h.WriteInt(0)
	}

	entry.Ttl = Pointer(now)
	return h.WriteInt(1)
}

func (h *ClientHandler) cmdTtl(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("ttl"))
	}

	key := bstring(args[0])

	entry, ok := lives(db, key)
	if !ok {
		return h.WriteInt(-2) // doesn't exist
	}

	if entry.Ttl == nil {
		return h.WriteInt(-1) // no expiry set
	}

	remaining := entry.Ttl.Sub(time.Now()).Seconds()
	return h.WriteInt64(int64(remaining))
}

func (h *ClientHandler) incr_decrByDelta(db *store.Database[string], args [][]byte, delta int64) error {

	key := bstring(copyBytes(args[0]))
	entry, ok := db.Get(key)
	if !ok {
		db.Set(key, store.NewStringEntry(Int64ToBytes(delta), nil))
		return h.WriteInt64(delta)
	}

	if entry.IsExpired(time.Now()) {
		// Logically absent: start from zero, no TTL.
		db.Set(key, store.NewStringEntry(Int64ToBytes(delta), nil))
		return h.WriteInt64(delta)
	}

	typedEntry, ok := entry.AsStr()
	if !ok {
		return h.WriteWrongTypeError()
	}

	current, err := ParseInt64(typedEntry.Value)
	if err != nil {
		return h.WriteError(err)
	}

	next, ok := Add64(current, delta)
	if !ok {
		return h.WriteError(ErrIncrDecrOverflow)
	}

	typedEntry.Value = Int64ToBytes(next)
	return h.WriteInt64(next)
}

func (h *ClientHandler) cmdIncr(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("incr"))
	}

	return h.incr_decrByDelta(db, args, 1)
}

func (h *ClientHandler) cmdDecr(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("decr"))
	}

	return h.incr_decrByDelta(db, args, -1)
}

func (h *ClientHandler) cmdIncrBy(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(wrongArgsNum("incrby"))
	}

	delta, err := ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}

	return h.incr_decrByDelta(db, args, delta)
}
func (h *ClientHandler) cmdDecrBy(db *store.Database[string], args [][]byte) error {
	if len(args) != 2 {
		return h.WriteError(wrongArgsNum("decrby"))
	}

	delta, err := ParseInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}

	return h.incr_decrByDelta(db, args, -delta)
}

func (h *ClientHandler) cmdMGet(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(wrongArgsNum("mget"))
	}

	h.WriteArrayHeader(len(args))
	for _, arg := range args {
		if h.werr != nil {
			return h.werr
		}
		key := bstring(arg)
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

	return h.werr
}

// Each pair is written atomically, but the batch as a whole is NOT atomic:
// another connection can observe some keys set and others not yet.
// Like SET, MSET discards any existing TTL.
func (h *ClientHandler) cmdMSet(db *store.Database[string], args [][]byte) error {
	if len(args) != 0 && len(args)%2 != 0 {
		return h.WriteError(wrongArgsNum("mset"))
	}

	pairs := make(map[string]*store.Entry)

	for pair := range slices.Chunk(args, 2) {
		key := bstring(copyBytes(pair[0]))
		value := copyBytes(pair[1])
		pairs[key] = store.NewStringEntry(value, nil)
	}

	db.MSet(pairs)
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdGetDel(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("getdel"))
	}

	key := bstring(args[0])
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
		return h.WriteError(wrongArgsNum("delex"))
	}

	key := bstring(args[0])
	condition := &SetCondition{kind: SET_ALWAYS}

	if len(args) == 1 {
		condition.kind = SET_ALWAYS
	} else if len(args) == 3 {
		option := bstring(bytesToLower(args[1]))
		operand := args[2]

		if option != "ifeq" && option != "ifne" && option != "ifdeq" && option != "ifdne" {
			return h.WriteError(ErrSyntaxError)
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
			return h.WriteError(ErrSyntaxError)
		}
		condition.cmp_value = operand
	} else {
		return h.WriteError(ErrSyntaxError)
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
		return h.WriteError(wrongArgsNum("getex"))
	}

	key := bstring(args[0])
	ttl := &SetTTL{action: SET_TTL_KEEP}

	i := 1
	for i < len(args) {
		opt := bstring(bytesToLower(args[i]))
		if opt == "persist" {
			if ttl.kind != nil && *ttl.kind != SET_TTL_PERSIST {
				return h.WriteError(ErrSyntaxError)
			}
			ttl.kind = Pointer(SET_TTL_PERSIST)
			ttl.action = SET_TTL_CLEAR
			i += 1
		} else {
			k := ttl_value_option(opt)
			if k != nil {
				if i+1 >= len(args) {
					return h.WriteError(ErrSyntaxError)
				}

				if ttl.kind != nil && *ttl.kind != *k {
					return h.WriteError(ErrSyntaxError)
				}

				n, err := ParseInt64(args[i+1])
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
				return h.WriteError(ErrSyntaxError)
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
		return h.WriteError(wrongArgsNum("strlen"))
	}

	key := bstring(args[0])
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
		return h.WriteError(wrongArgsNum("append"))
	}

	key := bstring(copyBytes(args[0]))
	suffix := copyBytes(args[1])

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
	if nl > h.maxBulkLength {
		return h.WriteError(ErrProtoInvalidBulkLength)
	}

	var buff bytes.Buffer
	buff.Write(strEntry.Value)
	buff.Write(suffix)
	strEntry.Value = buff.Bytes()

	return h.WriteInt(nl)
}

func (h *ClientHandler) cmdDigest(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("digest"))
	}

	key := bstring(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return h.WriteNullString()
	}

	strEntry, tok := entry.AsStr()
	if !tok {
		return h.WriteWrongTypeError()
	}

	digest := BytesToXXH3hex(strEntry.Value)
	return h.WriteString(digest)
}

func (h *ClientHandler) cmdType(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("type"))
	}

	key := bstring(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return h.WriteSimpleString("none")
	}

	return h.WriteSimpleString(entry.Value.TypeName())
}

func (h *ClientHandler) cmdDbSize(db *store.Database[string]) error {
	return h.WriteInt(db.Size())
}

func (h *ClientHandler) cmdFlushDb(db *store.Database[string]) error {
	db.Clear()
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdFlushAll(st *store.Store) error {
	for _, db := range st.DB {
		db.Clear()
	}
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdSelect(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("select"))
	}

	n, err := ParseInt32(args[0])
	if err != nil {
		return h.WriteError(err)
	}

	if n < 0 || n >= store.DB_NUM {
		return h.WriteError(fmt.Errorf("DB index is out of range"))
	}

	h.currentDB = int(n)
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdQuit() error {
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdCommand() error {
	return h.WriteSimpleString("OK")
}

func lives(db *store.Database[string], key string) (*store.Entry, bool) {
	entry, exists := db.Get(key)
	if !exists {
		return nil, false
	}

	if entry.IsExpired(time.Now()) {
		// it's expired, remove it
		db.Remove(key)
		return nil, false
	}

	return entry, true
}
