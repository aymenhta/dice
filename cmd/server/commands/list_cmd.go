package commands

import (
	"errors"

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
