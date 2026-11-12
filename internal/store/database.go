package store

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/utils"
)

type Database struct {
	sync.RWMutex
	Body map[string]*Entry
}

func NewDB[V any]() *Database {
	return &Database{
		Body: make(map[string]*Entry),
	}
}

func (db *Database) Set(key string, value *Entry) {
	db.Lock()
	db.Body[key] = value
	db.Unlock()
}

func (db *Database) Get(key string) (*Entry, bool) {
	db.RLock()
	v, ok := db.Body[key]
	db.RUnlock()
	return v, ok
}

// not safe for concurrent access
func (db *Database) getOrExpire(key string, t time.Time) (*Entry, bool) {
	entry, exists := db.Body[key]
	if !exists {
		return nil, false
	}

	if entry.IsExpired(t) {
		// it's expired, remove it
		delete(db.Body, key)
		return nil, false
	}

	return entry, true
}

func (db *Database) Remove(key string) {
	db.Lock()
	delete(db.Body, key)
	db.Unlock()
}

func (db *Database) RemoveHashFields(key string, fields []string, now time.Time) (int, error) {
	db.Lock()
	defer db.Unlock()
	removed := 0

	entry, ok := db.Body[key]
	if !ok {
		return 0, nil
	}

	if entry.IsExpired(now) {
		delete(db.Body, key)
		return 0, nil
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return 0, utils.ErrWrongTypeErr
	}

	for _, field := range fields {
		_, ok := hashE.Value[field]
		if ok {
			delete(hashE.Value, field)
			removed += 1
		}
	}

	if len(hashE.Value) == 0 {
		delete(db.Body, key)
	}

	return removed, nil
}

func (db *Database) IncrByHashFields(key string, field string, delta int64, now time.Time) (int64, error) {
	db.Lock()
	defer db.Unlock()

	entry, ok := db.Body[key]
	if !ok || entry.IsExpired(now) {
		h := make(map[string]*HashField)
		h[field] = NewHashField(utils.Int64ToBytes(delta))
		db.Body[key] = NewHashEntry(h, dsa.None[time.Time]())
		return delta, nil
	}

	hashE, ok := entry.AsHash()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}

	var current int64
	hf, ok := hashE.Value[field]
	if !ok || hf.IsExpired(now) {
		current = 0
	} else {
		hfv, err := utils.ParseInt64(hf.Value)
		if err != nil {
			return -1, fmt.Errorf("hash value is not an integer")
		}
		current = hfv
	}

	next, ok := utils.Add64(current, delta)
	if !ok {
		return -1, fmt.Errorf("increment or decrement would overflow")
	}

	hf.Value = utils.Int64ToBytes(next)
	return next, nil
}

func (db *Database) BatchRemove(keys []string) int64 {
	removed := int64(0)
	db.Lock()
	for _, key := range keys {
		_, ok := db.Body[key]
		if ok {
			delete(db.Body, key)
			removed += 1
		}
	}
	db.Unlock()
	// db.debugMap()
	return removed
}

func (db *Database) MSet(pairs map[string]*Entry) {
	db.Lock()
	for k, v := range pairs {
		db.Body[k] = v
	}
	db.Unlock()
}

func (db *Database) GetAndRemove(key string) ([]byte, bool, bool) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	entry, ok := db.Body[key]
	if !ok {
		return nil, false, false
	}

	if entry.IsExpired(now) {
		delete(db.Body, key)
		return nil, false, false
	}

	typedEntry, tok := entry.AsStr()
	if !tok {
		return nil, true, false
	}

	value := copyBytes(typedEntry.Value)
	delete(db.Body, key)
	return value, true, true
}

func (db *Database) Size() int {
	db.RLock()
	defer db.RUnlock()
	return len(db.Body)
}

func (db *Database) Clear() {
	db.Lock()
	clear(db.Body)
	defer db.Unlock()
}

func (db *Database) LPush(key string, elems [][]byte, mustExists bool) (int, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		if mustExists {
			return 0, nil
		}
		var dq dsa.Deque[[]byte]
		for _, el := range elems {
			dq.PushFront(el)
		}
		db.Body[key] = NewListEntry(&dq, dsa.None[time.Time]())
		return dq.Len(), nil
	}

	le, ok := e.AsList()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}
	for _, el := range elems {
		le.Value.PushFront(el)
	}

	return le.Value.Len(), nil
}

func (db *Database) RPush(key string, elems [][]byte, mustExists bool) (int, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		if mustExists {
			return 0, nil
		}
		var dq dsa.Deque[[]byte]
		for _, el := range elems {
			dq.PushBack(el)
		}
		db.Body[key] = NewListEntry(&dq, dsa.None[time.Time]())
		return dq.Len(), nil
	}

	le, ok := e.AsList()
	if !ok {
		return -1, utils.ErrWrongTypeErr
	}
	for _, el := range elems {
		le.Value.PushBack(el)
	}

	return le.Value.Len(), nil
}

func (db *Database) LPop(key string, count dsa.Option[int64]) ([][]byte, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		return nil, utils.ErrNoSuchKey
	}

	le, ok := e.AsList()
	if !ok {
		return nil, utils.ErrWrongTypeErr
	}

	if le.Value.IsEmpty() {
		return nil, utils.ErrNoSuchKey
	}

	if count.IsSome() {
		c := count.OrEmpty()
		response := make([][]byte, 0, c)
		i := int64(0)
		for !le.Value.IsEmpty() && i < c {
			x, _ := le.Value.PopFront()
			response = append(response, x)
			i += 1
		}
		return response, nil
	}

	x, _ := le.Value.PopFront()
	return [][]byte{x}, nil
}

func (db *Database) RPop(key string, count dsa.Option[int64]) ([][]byte, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		return nil, utils.ErrNoSuchKey
	}

	le, ok := e.AsList()
	if !ok {
		return nil, utils.ErrWrongTypeErr
	}

	if le.Value.IsEmpty() {
		return nil, utils.ErrNoSuchKey
	}

	if count.IsSome() {
		c := count.OrEmpty()
		response := make([][]byte, 0, c)
		i := int64(0)
		for !le.Value.IsEmpty() && i < c {
			x, _ := le.Value.PopBack()
			response = append(response, x)
			i += 1
		}
		return response, nil
	}

	x, _ := le.Value.PopBack()
	return [][]byte{x}, nil
}

func (db *Database) LRem(key string, count int, elem []byte) (int, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		return 0, utils.ErrNoSuchKey
	}

	le, ok := e.AsList()
	if !ok {
		return 0, utils.ErrWrongTypeErr
	}

	if le.Value.IsEmpty() {
		return 0, utils.ErrNoSuchKey
	}

	r := le.Value.RemoveOcc(count, func(v []byte) bool { return bytes.Equal(v, elem) })
	return r, nil
}

func (db *Database) LSet(key string, index int, elem []byte) error {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	e, ok := db.getOrExpire(key, now)
	if !ok {
		return utils.ErrNoSuchKey
	}

	le, ok := e.AsList()
	if !ok {
		return utils.ErrWrongTypeErr
	}

	if index < 0 {
		index = le.Value.Len() + index
	}

	if ok := le.Value.SetAt(index, elem); !ok {
		return utils.ErrIndexOutOfRange
	}
	return nil
}

func (db *Database) LMove(sk, dk, from, to string) ([]byte, error) {
	now := time.Now()
	db.Lock()
	defer db.Unlock()

	sourceEntry, ok := db.getOrExpire(sk, now)
	if !ok {
		return nil, utils.ErrNoSuchKey
	}

	sList, ok := sourceEntry.AsList()
	if !ok {
		return nil, utils.ErrWrongTypeErr
	}

	if sList.Value.IsEmpty() {
		return nil, utils.ErrNoSuchKey
	}

	destEntry, ok := db.getOrExpire(dk, now)
	if !ok {
		destEntry = NewListEntry(dsa.NewDeque[[]byte](), dsa.None[time.Time]())
		db.Body[dk] = destEntry
	}

	dList, ok := destEntry.AsList()
	if !ok {
		return nil, utils.ErrWrongTypeErr
	}

	var sv []byte
	switch from {
	case "right":
		x, _ := sList.Value.PopBack()
		sv = utils.CopyBytes(x)
	case "left":
		x, _ := sList.Value.PopFront()
		sv = utils.CopyBytes(x)
	default:
		return nil, utils.ErrSyntaxError
	}

	switch to {
	case "right":
		dList.Value.PushBack(sv)
	case "left":
		dList.Value.PushFront(sv)
	default:
		return nil, utils.ErrSyntaxError
	}

	if sList.Value.IsEmpty() {
		delete(db.Body, sk)
	}

	return sv, nil
}

func (db *Database) debugMap() {
	println("====================================================")
	fmt.Println("key -> value")
	for key, value := range db.Body {
		fmt.Printf("%s -> %+v\n", key, value)
	}
	println("====================================================")
}

func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
