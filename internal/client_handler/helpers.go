package client_handler

import (
	"time"

	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

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
		return false, resp.ErrSyntaxError
	}
}
