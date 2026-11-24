package commands

import (
	"time"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
)

func (mc *RespCommandManager) cmdSAdd(db *store.Database, args [][]byte) error {
	k := utils.BStr(utils.CopyBytes(args[0]))
	vals := make([]string, 0, len(args)-1)
	for _, v := range args[1:] {
		vals = append(vals, utils.BStr(utils.CopyBytes(v)))
	}

	e, ok := lives(db, k)
	if !ok {
		set := dsa.NewSet[string]()
		n := set.AddFromSlice(vals)
		db.Set(k, store.NewSetEntry(set, dsa.None[time.Time]()))
		return mc.WriteInt(n)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}
	db.Lock()
	n := se.Value.AddFromSlice(vals)
	db.Unlock()
	return mc.WriteInt(n)
}
