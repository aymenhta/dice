package commands

import (
	"fmt"
	"io"

	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

type RespCommandManager struct {
	resp.RespReadWriter
	currentDB int
}

func NewManager(rw io.ReadWriter) *RespCommandManager {
	return &RespCommandManager{
		RespReadWriter: *resp.NewRespReadWriter(rw),
		currentDB:      0,
	}
}

func (cm *RespCommandManager) Dispatch(store *store.Store, args [][]byte) error {
	cmd := utils.BStr(utils.BytesToLower(args[0]))
	args = args[1:]

	db := store.DB[cm.currentDB]

	switch cmd {
	// string commands
	case "set":
		return cm.cmdSet(db, args)
	case "get":
		return cm.cmdGet(db, args)
	case "del":
		return cm.cmdDel(db, args)
	case "incr":
		return cm.cmdIncr(db, args)
	case "decr":
		return cm.cmdDecr(db, args)
	case "incrby":
		return cm.cmdIncrBy(db, args)
	case "decrby":
		return cm.cmdDecrBy(db, args)
	case "mget":
		return cm.cmdMGet(db, args)
	case "mset":
		return cm.cmdMSet(db, args)
	case "getdel":
		return cm.cmdGetDel(db, args)
	case "delex":
		return cm.cmdDelEx(db, args)
	case "getex":
		return cm.cmdGetEx(db, args)
	case "strlen":
		return cm.cmdStrLen(db, args)
	case "append":
		return cm.cmdAppend(db, args)
	case "digest":
		return cm.cmdDigest(db, args)
	case "getrange":
		if len(args) != 3 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdGetRange(db, args)
	case "setrange":
		if len(args) != 3 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdSetRange(db, args)

	// hash commands
	case "hset":
		return cm.cmdHSet(db, args)
	case "hget":
		return cm.cmdHGet(db, args)
	case "hgetall":
		if len(args) != 1 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHGetAll(db, args)
	case "hdel":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHDel(db, args)
	case "hexists":
		if len(args) != 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHExists(db, args)
	case "hlen":
		if len(args) != 1 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHLen(db, args)
	case "hmget":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHMGet(db, args)
	case "hkeys":
		if len(args) != 1 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHKeysOrVals(db, args, true)
	case "hvals":
		if len(args) != 1 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHKeysOrVals(db, args, false)
	case "hincrby":
		if len(args) != 3 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHIncrBy(db, args)
	case "hexpire":
		if len(args) < 5 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdHExpire(db, args)

	// list commands
	case "lpush":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLPush(db, args)
	case "rpush":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdRPush(db, args)
	case "lpushx":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLPushx(db, args)
	case "rpushx":
		if len(args) < 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdRPushx(db, args)
	case "lpop":
		if len(args) < 1 || len(args) > 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLPop(db, args)
	case "rpop":
		if len(args) < 1 || len(args) > 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdRPop(db, args)
	case "lrange":
		if len(args) != 3 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLRange(db, args)
	case "llen":
		if len(args) != 1 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLLen(db, args)
	case "lindex":
		if len(args) != 2 {
			return cm.WriteError(utils.WrongArgsNum(cmd))
		}
		return cm.cmdLIndex(db, args)

	// general commands
	case "ping":
		return cm.cmdPing(args)
	case "echo":
		return cm.cmdEcho(args)
	case "exists":
		return cm.cmdExists(db, args)
	case "ttl":
		return cm.cmdTtl(db, args)
	case "expire":
		return cm.cmdExpire(db, args)
	case "type":
		return cm.cmdType(db, args)
	case "dbsize":
		return cm.cmdDbSize(db)
	case "flushdb":
		return cm.cmdFlushDb(db)
	case "flushall":
		return cm.cmdFlushAll(store)
	case "select":
		return cm.cmdSelect(db, args)
	case "command":
		return cm.cmdCommand()
	case "quit":
		return cm.cmdQuit()
	default:
		return cm.WriteError(fmt.Errorf("unkown command '%s'", cmd))
	}
}
