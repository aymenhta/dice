package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/store"
	"github.com/aymenhta/dice/internal/utils"
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

type AuthCmdArgs struct {
	username string
	password string
}

type HelloCmdArgs struct {
	protoV     dsa.Option[int]
	auth       dsa.Option[AuthCmdArgs]
	clientName dsa.Option[string]
}

func parseHelloCmdArgs(args [][]byte) (*HelloCmdArgs, error) {
	x := &HelloCmdArgs{
		protoV:     dsa.None[int](),
		auth:       dsa.None[AuthCmdArgs](),
		clientName: dsa.None[string](),
	}

	if len(args) == 0 {
		return x, nil
	}

	if len(args) >= 1 {
		n, err := utils.ParseUInt32(args[0])
		if err != nil {
			return nil, err
		}
		if n != 2 && n != 3 {
			return nil, utils.ErrNoProtoErr
		}
		x.protoV = dsa.Some(int(n))
	}

	i := 1
	for i < len(args) {
		switch utils.BStrLowered(args[i]) {
		case "auth":
			if i+1 >= len(args) || i+2 >= len(args) {
				return nil, utils.ErrSyntaxError
			}
			x.auth = dsa.Some(AuthCmdArgs{
				username: utils.BStr(args[i+1]),
				password: utils.BStr(args[i+2]),
			})
			i += 3
		case "setname":
			if i+1 >= len(args) {
				return nil, utils.ErrSyntaxError
			}
			x.clientName = dsa.Some(utils.BStr(args[i+1]))
			i += 2
		default:
			return nil, utils.ErrSyntaxError
		}
	}

	return x, nil
}

// returns the following list:
// - server: The server software name, usually redis.
// - version: The server’s version.
// - proto: The RESP protocol version used by this connection—2 or 3.
// - id: The unique ID of this client connection on the server.
// - mode: The server’s operating mode, such as standalone, sentinel, or cluster.
// - role: The server’s replication role, typically master or replica.
// - modules: Modules loaded on the server, if any; otherwise an empty list.
//
// TODO; support user auth
func (mc *RespCommandManager) cmdHello(args [][]byte) error {
	x, err := parseHelloCmdArgs(args)
	if err != nil {
		switch {
		case errors.Is(err, utils.ErrNoProtoErr):
			return mc.WriteNoProtoError()
		default:
			return mc.WriteError(err)
		}
	}

	if x.auth.IsSome() {
		return mc.WriteError(errors.New("auth is not supported yet"))
	}

	if x.protoV.IsSome() {
		switch x.protoV.OrEmpty() {
		case 2:
			mc.SetRESP2Compat(true)
		case 3:
			mc.SetRESP2Compat(false)
		default:
			return mc.WriteNoProtoError()
		}
	}

	mc.clientName = x.clientName

	if mc.RESP2Compat() {
		mc.WriteArrayHeader(14)
	} else {
		mc.WriteMapHeader(7)
	}
	mc.WriteString("server")
	mc.WriteString("dice")
	mc.WriteString("version")
	mc.WriteString("0.0.1")
	mc.WriteString("proto")
	if mc.RESP2Compat() {
		mc.WriteInt64(2)
	} else {
		mc.WriteInt64(3)
	}
	mc.WriteString("id")
	mc.WriteInt64(int64(mc.clientID))
	mc.WriteString("mode")
	mc.WriteString("standalone")
	mc.WriteString("role")
	mc.WriteString("master")
	mc.WriteString("modules")
	mc.WriteEmptyArray()
	return mc.Werr()
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
