package commands

import (
	"fmt"
	"time"

	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

func (cm *RespCommandManager) cmdPing(args [][]byte) error {
	if len(args) > 1 {
		return cm.WriteError(utils.WrongArgsNum("ping"))
	}

	if len(args) == 1 {
		return cm.WriteBytes(args[0])
	}

	return cm.WriteSimpleString("PONG")
}

func (cm *RespCommandManager) cmdEcho(args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteError(utils.WrongArgsNum("echo"))
	}
	return cm.WriteBytes(args[0])
}

func (cm *RespCommandManager) cmdExists(db *store.Database, args [][]byte) error {
	if len(args) == 0 {
		return cm.WriteError(utils.WrongArgsNum("exists"))
	}

	count := int64(0)
	for i := 0; i < len(args); i++ {
		key := utils.BStr(args[i])
		if _, ok := lives(db, key); ok {
			count += 1
		}
	}

	return cm.WriteInt64(count)
}

func (cm *RespCommandManager) cmdDel(db *store.Database, args [][]byte) error {
	if len(args) == 0 {
		return cm.WriteError(utils.WrongArgsNum("del"))
	}

	toBeDeleted := make([]string, 0)

	for i := 0; i < len(args); i++ {
		key := utils.BStr(args[i])
		toBeDeleted = append(toBeDeleted, key)
	}

	removed := db.BatchRemove(toBeDeleted)
	return cm.WriteInt64(removed)
}

func (cm *RespCommandManager) cmdExpire(db *store.Database, args [][]byte) error {
	if len(args) < 2 || len(args) > 3 {
		return cm.WriteError(utils.WrongArgsNum("expire"))
	}

	key := utils.BStr(args[0])
	secs, err := utils.ParseInt64(args[1])
	if err != nil {
		return cm.WriteError(err)
	}
	if secs < 0 {
		return cm.WriteError(utils.InvalidExpire("expire"))
	}

	entry, ok := db.Get(key)
	if !ok {
		return cm.WriteInt(0)
	}

	now := time.Now().Add(time.Duration(secs) * time.Second)
	var condition []byte = nil
	if len(args) == 3 {
		condition = args[2]
	}

	allowed, err := isCmdExpireAllowed(condition, now, entry.Ttl)
	if err != nil {
		return cm.WriteError(err)
	}

	if !allowed {
		return cm.WriteInt(0)
	}

	entry.Ttl = dsa.Some(now)
	return cm.WriteInt(1)
}

func (cm *RespCommandManager) cmdTtl(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteError(utils.WrongArgsNum("ttl"))
	}

	key := utils.BStr(args[0])

	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteInt(-2) // doesn't exist
	}

	if entry.Ttl.IsNone() {
		return cm.WriteInt(-1) // no expiry set
	}

	remaining := entry.Ttl.OrEmpty().Sub(time.Now()).Seconds()
	return cm.WriteInt64(int64(remaining))
}

func (cm *RespCommandManager) cmdType(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteError(utils.WrongArgsNum("type"))
	}

	key := utils.BStr(args[0])
	entry, ok := lives(db, key)
	if !ok {
		return cm.WriteSimpleString("none")
	}

	return cm.WriteSimpleString(entry.Value.TypeName())
}

// FIXME: it counts expired keys as well
func (cm *RespCommandManager) cmdDbSize(db *store.Database) error {
	return cm.WriteInt(db.Size())
}

// TODO: add support for optional argumant ASYNC | SYNC
func (cm *RespCommandManager) cmdFlushDb(db *store.Database) error {
	db.Clear()
	return cm.WriteSimpleString("OK")
}

// TODO: add support for optional argumant ASYNC | SYNC
func (cm *RespCommandManager) cmdFlushAll(st *store.Store) error {
	for _, db := range st.DB {
		db.Clear()
	}
	return cm.WriteSimpleString("OK")
}

func (cm *RespCommandManager) cmdSelect(db *store.Database, args [][]byte) error {
	if len(args) != 1 {
		return cm.WriteError(utils.WrongArgsNum("select"))
	}

	n, err := utils.ParseInt32(args[0])
	if err != nil {
		return cm.WriteError(err)
	}

	if n < 0 || n >= store.DB_NUM {
		return cm.WriteError(fmt.Errorf("DB index is out of range"))
	}

	cm.currentDB = int(n)
	return cm.WriteSimpleString("OK")
}

func (cm *RespCommandManager) cmdQuit() error {
	return cm.WriteSimpleString("OK")
}

func (cm *RespCommandManager) cmdCommand() error {
	return cm.WriteSimpleString("OK")
}
