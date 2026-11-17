package commands

import (
	"errors"
	"fmt"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
)

func (mc *RespCommandManager) cmdLPush(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	elems := make([][]byte, 0, len(args)-1)
	for _, v := range args[1:] {
		elems = append(elems, utils.CopyBytes(v))
	}

	n, err := db.LPush(key, elems, false)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt(n)
}

func (mc *RespCommandManager) cmdRPush(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	elems := make([][]byte, 0, len(args)-1)
	for _, v := range args[1:] {
		elems = append(elems, utils.CopyBytes(v))
	}

	n, err := db.RPush(key, elems, false)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt(n)
}

func (mc *RespCommandManager) cmdLPushx(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	elems := make([][]byte, 0, len(args)-1)
	for _, v := range args[1:] {
		elems = append(elems, utils.CopyBytes(v))
	}

	n, err := db.LPush(key, elems, true)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt(n)
}

func (mc *RespCommandManager) cmdRPushx(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	elems := make([][]byte, 0, len(args)-1)
	for _, v := range args[1:] {
		elems = append(elems, utils.CopyBytes(v))
	}

	n, err := db.RPush(key, elems, true)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteInt(n)
}

func (mc *RespCommandManager) cmdLPop(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	count := dsa.None[int64]()
	if len(args) == 2 {
		c, err := utils.ParseInt64(args[1])
		if err != nil {
			return mc.WriteError(err)
		}
		if c < 0 {
			return mc.WriteError(fmt.Errorf("value is out of range, must be positive"))
		}
		count = dsa.Some(c)
	}

	b, err := db.LPop(key, count)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		case errors.Is(err, utils.ErrNoSuchKey):
			return mc.WriteNullString()
		default:
			return mc.WriteError(err)
		}
	}

	if count.IsNone() {
		if len(b) != 1 {
			return mc.WriteError(fmt.Errorf("Server Error"))
		}
		return mc.WriteBytes(b[0])
	}

	return mc.WriteArrayBytes(b)
}

func (mc *RespCommandManager) cmdRPop(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	count := dsa.None[int64]()
	if len(args) == 2 {
		c, err := utils.ParseInt64(args[1])
		if err != nil {
			return mc.WriteError(err)
		}
		if c < 0 {
			return mc.WriteError(fmt.Errorf("value is out of range, must be positive"))
		}
		count = dsa.Some(c)
	}

	b, err := db.RPop(key, count)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		case errors.Is(err, utils.ErrNoSuchKey):
			return mc.WriteNullString()
		default:
			return mc.WriteError(err)
		}
	}

	if count.IsNone() {
		if len(b) != 1 {
			return mc.WriteError(fmt.Errorf("Server Error"))
		}
		return mc.WriteBytes(b[0])
	}

	return mc.WriteArrayBytes(b)
}

func (mc *RespCommandManager) cmdLRange(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	start, err := utils.ParseInt64(args[1])
	if err != nil {
		return mc.WriteError(err)
	}
	end, err := utils.ParseInt64(args[2])
	if err != nil {
		return mc.WriteError(err)
	}

	entry, ok := lives(db, key)
	if !ok {
		return mc.WriteEmptyArray()
	}

	le, ok := entry.AsList()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	s, e, ok := getrangeBounds(le.Value.Len(), start, end)
	if !ok {
		return mc.WriteEmptyArray()
	}

	return mc.WriteArrayBytes(le.Value.Slice(int(s), int(e+1)))
}

func (mc *RespCommandManager) cmdLLen(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])

	e, ok := lives(db, key)
	if !ok {
		return mc.WriteInt64(0)
	}

	le, ok := e.AsList()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	return mc.WriteInt(le.Value.Len())
}

func (mc *RespCommandManager) cmdLIndex(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	n, err := utils.ParseInt64(args[1])
	if err != nil {
		return mc.WriteError(err)
	}
	index := int(n)

	e, ok := lives(db, key)
	if !ok {
		return mc.WriteInt64(0)
	}

	le, ok := e.AsList()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	len := le.Value.Len()

	if index < 0 {
		v, ok := le.Value.At(len + index)
		if !ok {
			return mc.WriteNullString()
		}
		return mc.WriteBytes(v)
	}

	v, ok := le.Value.At(index)
	if !ok {
		return mc.WriteNullString()
	}
	return mc.WriteBytes(v)
}

func (mc *RespCommandManager) cmdLRem(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	count, err := utils.ParseInt64(args[1])
	if err != nil {
		return mc.WriteError(err)
	}

	r, err := db.LRem(key, int(count), args[2])
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		case errors.Is(err, utils.ErrNoSuchKey):
			return mc.WriteInt64(0)
		default:
			return mc.WriteError(err)
		}
	}
	return mc.WriteInt(r)
}

func (mc *RespCommandManager) cmdLSet(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	index, err := utils.ParseInt64(args[1])
	if err != nil {
		return mc.WriteError(err)
	}
	elem := utils.CopyBytes(args[2])

	err = db.LSet(key, int(index), elem)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteOK()
}

type ListMoveKind uint8

const (
	MOVE_TO_LEFT ListMoveKind = iota
	MOVE_TO_RIGHT
)

func moveKindFromBytes(b []byte) (string, error) {
	s := utils.BStr(utils.BytesToLower(b))
	if s != "right" && s != "left" {
		return "", utils.ErrSyntaxError
	}
	return s, nil
}

func (mc *RespCommandManager) cmdLMove(db *store.Database, args [][]byte) error {
	skey := utils.BStr(args[0])
	dkey := utils.BStr(utils.CopyBytes(args[1]))
	whereFrom, err := moveKindFromBytes(args[2])
	if err != nil {
		return mc.WriteError(err)
	}
	whereTo, err := moveKindFromBytes(args[3])
	if err != nil {
		return mc.WriteError(err)
	}

	b, err := db.LMove(skey, dkey, whereFrom, whereTo)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		case errors.Is(err, utils.ErrNoSuchKey):
			return mc.WriteNullString()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteBytes(b)
}

func (mc *RespCommandManager) cmdLTrim(db *store.Database, args [][]byte) error {
	key := utils.BStr(args[0])
	start, err := utils.ParseInt64(args[1])
	if err != nil {
		return mc.WriteError(err)
	}
	stop, err := utils.ParseInt64(args[2])
	if err != nil {
		return mc.WriteError(err)
	}

	if err := db.LTrim(key, int(start), int(stop)); err != nil {
		switch {
		case errors.Is(err, utils.ErrWrongTypeErr):
			return mc.WriteWrongTypeError()
		default:
			return mc.WriteError(err)
		}
	}

	return mc.WriteOK()
}
