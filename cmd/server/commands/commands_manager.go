package commands

import (
	"fmt"
	"io"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/resp"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
)

type RespCommandManager struct {
	resp.RespReadWriter
	currentDB  int
	clientName dsa.Option[string]
	clientID   uint64
}

func NewManager(rw io.ReadWriter, clientID uint64) *RespCommandManager {
	return &RespCommandManager{
		RespReadWriter: *resp.NewRespReadWriter(rw),
		currentDB:      0,
		clientName:     dsa.None[string](),
		clientID:       clientID,
	}
}

func (mc *RespCommandManager) Dispatch(store *store.Store, args [][]byte) error {
	cmd := utils.BStr(utils.BytesToLower(args[0]))
	args = args[1:]

	db := store.DB[mc.currentDB]

	switch cmd {
	// string commands
	case "set":
		return mc.cmdSet(db, args)
	case "get":
		return mc.cmdGet(db, args)
	case "del":
		return mc.cmdDel(db, args)
	case "incr":
		return mc.cmdIncr(db, args)
	case "decr":
		return mc.cmdDecr(db, args)
	case "incrby":
		return mc.cmdIncrBy(db, args)
	case "decrby":
		return mc.cmdDecrBy(db, args)
	case "mget":
		return mc.cmdMGet(db, args)
	case "mset":
		return mc.cmdMSet(db, args)
	case "getdel":
		return mc.cmdGetDel(db, args)
	case "delex":
		return mc.cmdDelEx(db, args)
	case "getex":
		return mc.cmdGetEx(db, args)
	case "strlen":
		return mc.cmdStrLen(db, args)
	case "append":
		return mc.cmdAppend(db, args)
	case "digest":
		return mc.cmdDigest(db, args)
	case "getrange":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdGetRange(db, args)
	case "setrange":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSetRange(db, args)
	case "msetex":
		if len(args) < 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdMSetEx(db, args)
	case "msetnx":
		if len(args)%2 != 0 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdMSetNx(db, args)
	case "setnx":
		if len(args) != 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSetNx(db, args)

	// hash commands
	case "hset":
		return mc.cmdHSet(db, args)
	case "hget":
		return mc.cmdHGet(db, args)
	case "hgetall":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHGetAll(db, args)
	case "hdel":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHDel(db, args)
	case "hexists":
		if len(args) != 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHExists(db, args)
	case "hlen":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHLen(db, args)
	case "hmget":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHMGet(db, args)
	case "hkeys":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHKeysOrVals(db, args, true)
	case "hvals":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHKeysOrVals(db, args, false)
	case "hincrby":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHIncrBy(db, args)
	case "hexpire":
		if len(args) < 5 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdHExpire(db, args)

	// list commands
	case "lpush":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLPush(db, args)
	case "rpush":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdRPush(db, args)
	case "lpushx":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLPushx(db, args)
	case "rpushx":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdRPushx(db, args)
	case "lpop":
		if len(args) < 1 || len(args) > 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLPop(db, args)
	case "rpop":
		if len(args) < 1 || len(args) > 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdRPop(db, args)
	case "lrange":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLRange(db, args)
	case "llen":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLLen(db, args)
	case "lindex":
		if len(args) != 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLIndex(db, args)
	case "lrem":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLRem(db, args)
	case "lset":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLSet(db, args)
	case "lmove":
		if len(args) != 4 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLMove(db, args)
	case "ltrim":
		if len(args) != 3 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdLTrim(db, args)

	// set commands
	case "sadd":
		if len(args) < 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSAdd(db, args)
	case "smembers":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSMembers(db, args)
	case "sismember":
		if len(args) != 2 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSIsMember(db, args)
	case "scard":
		if len(args) != 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSCard(db, args)
	case "sinter":
		if len(args) < 1 {
			return mc.WriteError(utils.WrongArgsNum(cmd))
		}
		return mc.cmdSInter(db, args)

	// general commands
	case "ping":
		return mc.cmdPing(args)
	case "echo":
		return mc.cmdEcho(args)
	case "hello":
		return mc.cmdHello(args)
	case "exists":
		return mc.cmdExists(db, args)
	case "ttl":
		return mc.cmdTtl(db, args)
	case "expire":
		return mc.cmdExpire(db, args)
	case "type":
		return mc.cmdType(db, args)
	case "dbsize":
		return mc.cmdDbSize(db)
	case "flushdb":
		return mc.cmdFlushDb(db)
	case "flushall":
		return mc.cmdFlushAll(store)
	case "select":
		return mc.cmdSelect(db, args)
	case "command":
		return mc.cmdCommand()
	case "quit":
		return mc.cmdQuit()
	default:
		return mc.WriteError(fmt.Errorf("unkown command '%s'", cmd))
	}
}
