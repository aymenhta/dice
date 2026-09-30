package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"

	"aymenhta.com/dice/resp"
)

const MAX_DB_NUM = 16

type DB map[string][]byte

type Store [MAX_DB_NUM]DB

type DiceServer struct {
	port      string
	logger    *slog.Logger
	store     Store
	currentDB int
}

func NewDiceServer(port string, logger *slog.Logger) *DiceServer {
	// initialize store
	var store Store
	for i := range MAX_DB_NUM {
		store[i] = make(DB)
	}

	var srv DiceServer
	srv.port = port
	srv.logger = logger
	srv.store = store
	srv.currentDB = 0
	return &srv
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

		frame := srv.dispatch(buff[0:bytesRead])
		if _, err := resp.SerializeTo(conn, frame); err != nil {
			srv.logger.Error(err.Error())
		}
	}
	return nil
}

func (srv *DiceServer) dispatch(buff []byte) resp.Frame {
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
	db := srv.store[srv.currentDB]

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
			return wrongNumberOfArgumentError(cmd)
		}

		return resp.NewBulkString(args[0])
	}

	if strings.EqualFold(cmd, "set") {
		if len(args) != 2 {
			return wrongNumberOfArgumentError(cmd)
		}

		return set(db, args[0], args[1])
	}

	if strings.EqualFold(cmd, "get") {
		if len(args) != 1 {
			return wrongNumberOfArgumentError(cmd)
		}

		return get(db, args[0])
	}

	if strings.EqualFold(cmd, "command") {
		return resp.NewOkSimpleString()
	}

	return resp.NewError(fmt.Sprintf("unknown command '%s'", cmd))
}
