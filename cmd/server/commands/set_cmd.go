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

func (mc *RespCommandManager) cmdSMembers(db *store.Database, args [][]byte) error {
	k := utils.BStr(args[0])

	e, ok := lives(db, k)
	if !ok {
		if mc.RESP2Compat() {
			return mc.WriteEmptyArray()
		}
		return mc.WriteEmptySet()
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	if mc.RESP2Compat() {
		mc.WriteArrayHeader(se.Value.Size())
	} else {
		mc.WriteSetHeader(se.Value.Size())
	}

	for item := range se.Value.Items() {
		mc.WriteString(item)
	}

	return mc.Werr()
}

func (mc *RespCommandManager) cmdSIsMember(db *store.Database, args [][]byte) error {
	k := utils.BStr(args[0])
	v := utils.BStr(args[1])

	e, ok := lives(db, k)
	if !ok {
		return mc.WriteInt64(0)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	if !se.Value.Contains(v) {
		return mc.WriteInt64(0)
	}
	return mc.WriteInt64(1)
}

func (mc *RespCommandManager) cmdSCard(db *store.Database, args [][]byte) error {
	k := utils.BStr(args[0])

	e, ok := lives(db, k)
	if !ok {
		return mc.WriteInt64(0)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	return mc.WriteInt(se.Value.Size())
}

func (mc *RespCommandManager) cmdSInter(db *store.Database, args [][]byte) error {
	k := utils.BStr(args[0])
	e, ok := lives(db, k)
	if !ok {
		if mc.RESP2Compat() {
			return mc.WriteEmptyArray()
		}
		return mc.WriteEmptySet()
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	var out dsa.Set[string]

	if len(args) == 1 {
		out = se.Value.IntersectN()
	} else {
		others := make([]dsa.Set[string], 0, len(args)-1)
		for _, v := range args[1:] {
			e, ok := lives(db, utils.BStr(v))
			if !ok {
				if mc.RESP2Compat() {
					return mc.WriteEmptyArray()
				}
				return mc.WriteEmptySet()
			}
			se, ok := e.AsSet()
			if !ok {
				return mc.WriteWrongTypeError()
			}
			others = append(others, se.Value)
		}

		out = se.Value.IntersectN(others...)
	}

	if mc.RESP2Compat() {
		mc.WriteArrayHeader(out.Size())
	} else {
		mc.WriteSetHeader(out.Size())
	}

	for item := range out.Items() {
		mc.WriteString(item)
	}

	return mc.Werr()
}

func (mc *RespCommandManager) cmdSUnion(db *store.Database, args [][]byte) error {
	empty := func() error {
		if mc.RESP2Compat() {
			return mc.WriteEmptyArray()
		}
		return mc.WriteEmptySet()
	}
	var fst dsa.Set[string]

	k := utils.BStr(args[0])
	e, exists := lives(db, k)
	if !exists {
		if len(args) == 1 {
			return empty()
		}

		fst = dsa.NewSet[string]()
	} else {
		se, ok := e.AsSet()
		if !ok {
			return mc.WriteWrongTypeError()
		}
		fst = se.Value
	}

	var out dsa.Set[string]
	if len(args) == 1 {
		out = fst.UnionN()
	} else {
		others := make([]dsa.Set[string], 0, len(args)-1)
		for _, v := range args[1:] {
			e, ok := lives(db, utils.BStr(v))
			if !ok {
				continue
			}
			se, ok := e.AsSet()
			if !ok {
				return mc.WriteWrongTypeError()
			}
			others = append(others, se.Value)
		}

		out = fst.UnionN(others...)
	}

	if mc.RESP2Compat() {
		mc.WriteArrayHeader(out.Size())
	} else {
		mc.WriteSetHeader(out.Size())
	}

	for item := range out.Items() {
		mc.WriteString(item)
	}

	return mc.Werr()
}
