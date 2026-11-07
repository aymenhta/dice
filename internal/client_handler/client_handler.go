package client_handler

import (
	"fmt"
	"io"

	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

type ClientHandler struct {
	resp.RespReadWriter
	currentDB int
}

func New(rw io.ReadWriter) *ClientHandler {
	return &ClientHandler{
		RespReadWriter: *resp.NewRespReadWriter(rw),
		currentDB:      0,
	}
}

func (h *ClientHandler) Dispatch(store *store.Store, args [][]byte) error {
	cmd := utils.BStr(utils.BytesToLower(args[0]))
	args = args[1:]

	db := store.DB[h.currentDB]

	switch cmd {
	// string commands
	case "set":
		return h.cmdSet(db, args)
	case "get":
		return h.cmdGet(db, args)
	case "del":
		return h.cmdDel(db, args)
	case "incr":
		return h.cmdIncr(db, args)
	case "decr":
		return h.cmdDecr(db, args)
	case "incrby":
		return h.cmdIncrBy(db, args)
	case "decrby":
		return h.cmdDecrBy(db, args)
	case "mget":
		return h.cmdMGet(db, args)
	case "mset":
		return h.cmdMSet(db, args)
	case "getdel":
		return h.cmdGetDel(db, args)
	case "delex":
		return h.cmdDelEx(db, args)
	case "getex":
		return h.cmdGetEx(db, args)
	case "strlen":
		return h.cmdStrLen(db, args)
	case "append":
		return h.cmdAppend(db, args)
	case "digest":
		return h.cmdDigest(db, args)
	case "getrange":
		if len(args) != 3 {
			return h.WriteError(utils.WrongArgsNum(cmd))
		}
		return h.cmdGetRange(db, args)
	case "setrange":
		if len(args) != 3 {
			return h.WriteError(utils.WrongArgsNum(cmd))
		}
		return h.cmdSetRange(db, args)

	// hash commands
	case "hset":
		return h.cmdHSet(db, args)
	case "hget":
		return h.cmdHGet(db, args)

	// general commands
	case "ping":
		return h.cmdPing(args)
	case "echo":
		return h.cmdEcho(args)
	case "exists":
		return h.cmdExists(db, args)
	case "ttl":
		return h.cmdTtl(db, args)
	case "expire":
		return h.cmdExpire(db, args)
	case "type":
		return h.cmdType(db, args)
	case "dbsize":
		return h.cmdDbSize(db)
	case "flushdb":
		return h.cmdFlushDb(db)
	case "flushall":
		return h.cmdFlushAll(store)
	case "select":
		return h.cmdSelect(db, args)
	case "command":
		return h.cmdCommand()
	case "quit":
		return h.cmdQuit()
	default:
		return h.WriteError(fmt.Errorf("unkown command '%s'", cmd))
	}
}
