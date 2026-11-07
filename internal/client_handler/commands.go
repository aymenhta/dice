package client_handler

import (
	"fmt"
	"time"

	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

func (h *ClientHandler) cmdPing(args [][]byte) error {
	if len(args) > 1 {
		return h.WriteError(utils.WrongArgsNum("ping"))
	}

	if len(args) == 1 {
		return h.WriteBytes(args[0])
	}

	return h.WriteSimpleString("PONG")
}

func (h *ClientHandler) cmdEcho(args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("echo"))
	}
	return h.WriteBytes(args[0])
}

func (h *ClientHandler) cmdExists(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(utils.WrongArgsNum("exists"))
	}

	count := int64(0)
	for i := 0; i < len(args); i++ {
		key := utils.BStr(args[i])
		if _, ok := lives(db, key); ok {
			count += 1
		}
	}

	return h.WriteInt64(count)
}

func (h *ClientHandler) cmdDel(db *store.Database[string], args [][]byte) error {
	if len(args) == 0 {
		return h.WriteError(utils.WrongArgsNum("del"))
	}

	toBeDeleted := make([]string, 0)

	for i := 0; i < len(args); i++ {
		key := utils.BStr(args[i])
		toBeDeleted = append(toBeDeleted, key)
	}

	removed := db.BatchRemove(toBeDeleted)
	return h.WriteInt64(removed)
}

func (h *ClientHandler) cmdExpire(db *store.Database[string], args [][]byte) error {
	if len(args) < 2 || len(args) > 3 {
		return h.WriteError(utils.WrongArgsNum("expire"))
	}

	key := utils.BStr(args[0])
	secs, err := utils.ParseUInt64(args[1])
	if err != nil {
		return h.WriteError(err)
	}
	if secs < 0 {
		return h.WriteError(utils.InvalidExpire("expire"))
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

	allowed, err := isCmdExpireAllowed(condition, now, entry.Ttl)
	if err != nil {
		return h.WriteError(err)
	}

	if !allowed {
		return h.WriteInt(0)
	}

	entry.Ttl = utils.Pointer(now)
	return h.WriteInt(1)
}

func (h *ClientHandler) cmdTtl(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("ttl"))
	}

	key := utils.BStr(args[0])

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

func (h *ClientHandler) cmdType(db *store.Database[string], args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(utils.WrongArgsNum("type"))
	}

	key := utils.BStr(args[0])
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
		return h.WriteError(utils.WrongArgsNum("select"))
	}

	n, err := utils.ParseInt32(args[0])
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
