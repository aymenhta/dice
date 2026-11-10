package commands

import (
	"errors"
	"fmt"

	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

func (mc *RespCommandManager) cmdLPush(db *store.Database, args [][]byte) error {
	key := utils.BStr(utils.CopyBytes(args[0]))
	elems := make([][]byte, 0, len(args)-1)
	for _, v := range args[1:] {
		elems = append(elems, utils.CopyBytes(v))
	}

	n, err := db.LPush(key, elems)
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

	n, err := db.RPush(key, elems)
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
		case errors.Is(err, utils.ErrRecordDoesntExist):
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
		case errors.Is(err, utils.ErrRecordDoesntExist):
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

	return mc.WriteArrayBytes(le.Value.Slice(s, e))
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
