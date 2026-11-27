package commands

import (
	"errors"
	"math/rand/v2"
	"slices"
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

func (mc *RespCommandManager) cmdSInterStore(db *store.Database, args [][]byte) error {
	k := utils.BStr(args[1])
	e, ok := lives(db, k)
	if !ok {
		return mc.WriteInt64(0)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	var out dsa.Set[string]

	if len(args) == 2 {
		out = se.Value.IntersectN()
	} else {
		others := make([]dsa.Set[string], 0, len(args)-2)
		for _, v := range args[2:] {
			e, ok := lives(db, utils.BStr(v))
			if !ok {
				return mc.WriteInt64(0)
			}
			se, ok := e.AsSet()
			if !ok {
				return mc.WriteWrongTypeError()
			}
			others = append(others, se.Value)
		}

		out = se.Value.IntersectN(others...)
	}

	destKey := utils.BStr(utils.CopyBytes(args[0]))
	destE, ok := lives(db, destKey)
	if !ok {
		db.Set(destKey, store.NewSetEntry(out, dsa.None[time.Time]()))
	} else {
		se, ok := e.AsSet()
		if !ok {
			return mc.WriteWrongTypeError()
		}
		se.Value = out
		db.Update(destKey, destE)
	}

	return mc.WriteInt(out.Size())
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

func (mc *RespCommandManager) cmdSUnionStore(db *store.Database, args [][]byte) error {
	destKey := utils.BStr(utils.CopyBytes(args[0]))

	var fst dsa.Set[string]
	k := utils.BStr(args[1])
	e, exists := lives(db, k)
	if !exists {
		if len(args) == 2 {
			return mc.WriteInt64(0)
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
	if len(args) == 2 {
		out = fst.UnionN()
	} else {
		others := make([]dsa.Set[string], 0, len(args)-2)
		for _, v := range args[2:] {
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

	// store out to dest
	destEntry, ok := lives(db, destKey)
	if !ok {
		db.Set(destKey, store.NewSetEntry(out, dsa.None[time.Time]()))
	} else {
		se, ok := destEntry.AsSet()
		if !ok {
			return mc.WriteWrongTypeError()
		}
		se.Value = out
		db.Update(destKey, destEntry)
	}

	return mc.WriteInt(out.Size())
}

func (mc *RespCommandManager) cmdSRem(db *store.Database, args [][]byte) error {
	e, ok := lives(db, utils.BStr(args[0]))
	if !ok {
		return mc.WriteInt64(0)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	removed := 0
	db.Lock()
	for _, arg := range args[1:] {
		m := utils.BStr(arg)
		if !se.Value.Contains(m) {
			continue
		}
		se.Value.Remove(m)
		removed += 1
	}
	db.Unlock()
	return mc.WriteInt(removed)
}

func (mc *RespCommandManager) cmdSDiff(db *store.Database, args [][]byte) error {
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
		out = fst.DifferenceN()
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

		out = fst.DifferenceN(others...)
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

func (mc *RespCommandManager) cmdSDiffStore(db *store.Database, args [][]byte) error {
	var fst dsa.Set[string]

	k := utils.BStr(args[1])
	e, exists := lives(db, k)
	if !exists {
		if len(args) == 2 {
			return mc.WriteInt64(0)
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
	if len(args) == 2 {
		out = fst.DifferenceN()
	} else {
		others := make([]dsa.Set[string], 0, len(args)-2)
		for _, v := range args[2:] {
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

		out = fst.DifferenceN(others...)
	}

	destKey := utils.BStr(utils.CopyBytes(args[0]))
	destE, ok := lives(db, destKey)
	if !ok {
		db.Set(destKey, store.NewSetEntry(out, dsa.None[time.Time]()))
	} else {
		se, ok := destE.AsSet()
		if !ok {
			return mc.WriteWrongTypeError()
		}
		se.Value = out
		db.Update(destKey, destE)
	}

	return mc.WriteInt(out.Size())
}

func (mc *RespCommandManager) cmdSInterCard(db *store.Database, args [][]byte) error {
	nkeys, err := utils.ParseInt64(args[0])
	if err != nil {
		return mc.WriteError(err)
	}

	if nkeys < 1 {
		return mc.WriteError(errors.New("numkeys should be greater than 0"))
	}

	offset := nkeys + 1
	keys := make([]string, 0, nkeys)
	for _, arg := range args[1:offset] {
		keys = append(keys, utils.BStr(arg))
	}

	var limit int64
	if offset < int64(len(args)) {
		if utils.BStr(utils.BytesToLower(args[offset])) != "limit" || offset+1 >= int64(len(args)) {
			return mc.WriteError(utils.ErrSyntaxError)
		}
		l, err := utils.ParseInt64(args[offset+1])
		if err != nil {
			return mc.WriteError(err)
		}
		if l < 0 {
			return mc.WriteError(errors.New("LIMIT can't be negative"))
		}
		limit = l
	}

	e, ok := lives(db, keys[0])
	if !ok {
		return mc.WriteInt64(0)
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}

	others := make([]dsa.Set[string], 0, len(keys)-1)
	for i := 1; i < len(keys); i++ {
		oe, ok := lives(db, keys[i])
		if !ok {
			return mc.WriteInt64(0)
		}
		ose, ok := oe.AsSet()
		if !ok {
			return mc.WriteWrongTypeError()
		}
		others = append(others, ose.Value)
	}

	card := se.Value.LimitIntersectN(limit, others...)
	return mc.WriteInt64(card)
}

func (mc *RespCommandManager) cmdSRandMember(db *store.Database, args [][]byte) error {
	var countArg dsa.Option[int64]
	if len(args) == 2 {
		c, err := utils.ParseInt64(args[1])
		if err != nil {
			return mc.WriteError(err)
		}
		countArg = dsa.Some(c)
	} else {
		countArg = dsa.None[int64]()
	}

	key := utils.BStr(args[0])
	e, ok := lives(db, key)
	if !ok {
		if countArg.IsSome() {
			return mc.WriteEmptyArray()
		}
		return mc.WriteNullString()
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}
	setSize := int64(se.Value.Size())
	setItems := slices.Collect(se.Value.Items())
	if setSize < 1 {
		return mc.WriteEmptyArray()
	}

	// without the additional count argument, the command returns a randomly selected member
	if countArg.IsNone() {
		out := setItems[rand.Int64N(setSize)]
		return mc.WriteString(out)
	}

	count := countArg.OrEmpty()
	if count >= 0 {
		osize := min(count, setSize)
		mc.WriteArrayHeaderInt64(osize)
		for range osize {
			mc.WriteString(setItems[rand.Int64N(setSize)])
		}
		return mc.Werr()
	}

	count = -count
	mc.WriteArrayHeaderInt64(count)
	hi := max(count, setSize)
	if hi == setSize {
		for range count {
			mc.WriteString(setItems[rand.Int64N(setSize)])
		}
		return mc.Werr()
	} else {
		for range setSize {
			mc.WriteString(setItems[rand.Int64N(setSize)])
		}
		rest := count - setSize
		if rest > 0 {
			for range rest {
				mc.WriteString(setItems[rand.Int64N(setSize)])
			}
		}
		return mc.Werr()
	}
}

func (mc *RespCommandManager) cmdSPop(db *store.Database, args [][]byte) error {
	var countArg dsa.Option[int64]
	if len(args) == 2 {
		c, err := utils.ParseInt64(args[1])
		if err != nil {
			return mc.WriteError(err)
		}
		if c < 0 {
			return mc.WriteError(errors.New("value is out of range, must be positive"))
		}
		if c == 0 {
			return mc.WriteEmptyArray()
		}
		countArg = dsa.Some(c)
	} else {
		countArg = dsa.None[int64]()
	}

	key := utils.BStr(utils.CopyBytes(args[0]))
	e, ok := lives(db, key)
	if !ok {
		if countArg.IsSome() {
			return mc.WriteEmptyArray()
		}
		return mc.WriteNullString()
	}

	se, ok := e.AsSet()
	if !ok {
		return mc.WriteWrongTypeError()
	}
	setSize := int64(se.Value.Size())
	if setSize < 1 {
		return mc.WriteEmptyArray()
	}

	if countArg.IsNone() {
		setItems := slices.Collect(se.Value.Items())
		out := setItems[rand.IntN(len(setItems))]
		se.Value.Remove(out)
		db.Update(key, e)
		return mc.WriteString(out)
	}

	count := countArg.OrEmpty()
	lo := min(count, setSize)
	mc.WriteArrayHeaderInt64(lo)
	for range lo {
		target := rand.IntN(se.Value.Size())
		var i int
		for item := range se.Value.Items() {
			if i == target {
				se.Value.Remove(item)
				db.Update(key, e)
				mc.WriteString(item)
			}
			i += 1
		}
	}

	if se.Value.Size() == 0 {
		db.Remove(key)
	}

	return mc.Werr()
}

func (mc *RespCommandManager) cmdSMove(db *store.Database, args [][]byte) error {
	n, err := db.SMove(utils.BStr(args[0]), utils.BStr(args[1]), utils.BStr(args[2]))
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
