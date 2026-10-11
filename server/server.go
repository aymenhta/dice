package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"aymenhta.com/dice/resp"
)

const MAX_DB_NUM = 16

type Entry struct {
	value     []byte
	expiresAt atomic.Uint64
}

type HashMap map[string]*Entry

type DB struct {
	sync.RWMutex
	hashmap HashMap
}

type Store [MAX_DB_NUM]*DB

func newStore() Store {
	var store Store
	for i := range MAX_DB_NUM {
		store[i] = &DB{
			hashmap: make(HashMap),
		}
	}

	return store
}

type DiceServer struct {
	port   string
	logger *slog.Logger
	store  Store
}

func NewDiceServer(port string, logger *slog.Logger) *DiceServer {
	return &DiceServer{
		port:   port,
		logger: logger,
		store:  newStore(),
	}
}

func (srv *DiceServer) Start() error {
	ln, err := net.Listen("tcp", srv.port)
	if err != nil {
		srv.logger.Error(err.Error())
		return err
	}
	defer ln.Close()
	srv.logger.Info("Started listening...", "port", srv.port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			srv.logger.Error(err.Error())
			return err
		}
		srv.logger.Info("a new connection arrived", "addr", conn.RemoteAddr().String())

		go srv.handleClient(conn)
	}
}

func (srv *DiceServer) handleClient(conn net.Conn) error {
	defer conn.Close()

	buff := make([]byte, 4096)
	reader := bufio.NewReader(conn)
	currentDB := 0
	for {
		bytesRead, err := reader.Read(buff)
		if err != nil {
			if errors.Is(err, io.EOF) && bytesRead == 0 {
				srv.logger.Info("client disconnected gracefully.")
				return nil
			}
			srv.logger.Error(err.Error())
			return err
		}

		if bytesRead == 0 {
			break
		}

		frame := srv.dispatch(buff[0:bytesRead], &currentDB)
		if _, err := resp.SerializeTo(conn, frame); err != nil {
			srv.logger.Error(err.Error())
		}
	}
	return nil
}

func (srv *DiceServer) dispatch(buff []byte, currentDB *int) resp.Frame {
	input, _, err := resp.Parse(buff)
	if err != nil {
		return resp.NewError(err.Error())
	}

	array, ok := input.(*resp.Array)
	if !ok {
		return resp.NewError("ERR invalid request, expected non-empty array")
	}

	if len(array.Value) < 1 {
		return resp.NewError("ERR invalid request, expected non-empty array")
	}

	for _, itemKind := range array.ItemsKinds {
		if itemKind != resp.PAYLOAD_BULK_STRINGS {
			return resp.NewError("ERR invalid request, expected an array of bulk strings") // TODO: check if it's the right message
		}
	}

	cmd := string(array.Value[0])
	args := array.Value[1:]
	db := srv.store[*currentDB]

	if strings.EqualFold(cmd, "ping") {
		message := elementAt(args, 0)
		if len(message) < 1 {
			return &resp.SimpleString{Value: []byte("PONG")}
		} else {
			return &resp.BulkString{Value: message}
		}
	}

	if strings.EqualFold(cmd, "echo") {
		if len(args) != 1 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return resp.NewBulkString(args[0])
	}

	if strings.EqualFold(cmd, "set") {
		if len(args) != 2 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return set(db.hashmap, args[0], args[1])
	}

	if strings.EqualFold(cmd, "get") {
		if len(args) != 1 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return get(db.hashmap, args[0])
	}

	if strings.EqualFold(cmd, "del") {
		if len(args) == 0 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return del(db.hashmap, args)
	}

	if strings.EqualFold(cmd, "exists") {
		if len(args) == 0 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return exists(db.hashmap, args)
	}

	if strings.EqualFold(cmd, "incr") {
		if len(args) != 1 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return incr(db.hashmap, args[0], 1)
	}

	if strings.EqualFold(cmd, "decr") {
		if len(args) != 1 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return incr(db.hashmap, args[0], -1)
	}

	if strings.EqualFold(cmd, "dbsize") {
		return dbSize(db.hashmap)
	}

	if strings.EqualFold(cmd, "flushdb") {
		return flushDB(db.hashmap)
	}

	if strings.EqualFold(cmd, "flushall") {
		return flushAll(srv.store)
	}

	if strings.EqualFold(cmd, "select") {
		if len(args) != 1 {
			return wrongNumberOfArgumentErr(cmd)
		}

		return selectCMD(args[0], currentDB)
	}

	if strings.EqualFold(cmd, "command") {
		return resp.NewOkSimpleString()
	}

	return resp.NewError(fmt.Sprintf("unknown command '%s'", cmd))
}
