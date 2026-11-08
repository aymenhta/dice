package commands

import (
	"time"

	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

func lives(db *store.Database, key string) (*store.Entry, bool) {
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

func isCmdExpireAllowed(cond []byte, new_ttl time.Time, old_ttl *time.Time) (bool, error) {
	if cond == nil || len(cond) == 0 {
		return true, nil
	}

	switch utils.BStr(utils.BytesToLower(cond)) {
	case "nx":
		return old_ttl == nil, nil
	case "xx":
		return old_ttl != nil, nil
	case "gt":
		return old_ttl != nil && new_ttl.After(*old_ttl), nil
	case "lt":
		return old_ttl == nil || new_ttl.Before(*old_ttl), nil
	default:
		return false, utils.ErrSyntaxError
	}
}

// Maps GETRANGE's (possibly negative, inclusive) indices onto a concrete
// inclusive byte range. `false` = empty.
func getrangeBounds(len int, start, end int64) (int64, int64, bool) {
	lenI := int64(len)
	if start < 0 && end < 0 && start > end {
		return -1, -1, false
	}

	s := start
	e := end
	if start < 0 {
		s = lenI + start
	}
	if end < 0 {
		e = lenI + end
	}

	if s < 0 {
		s = 0
	}
	if e < 0 {
		e = 0
	}

	if e >= lenI {
		e = lenI - 1
	}
	if len == 0 || s > e {
		return -1, -1, false
	}

	return s, e, true
}
