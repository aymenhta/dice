package server

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"aymenhta.com/dice/command"
	"aymenhta.com/dice/diceio"
	"aymenhta.com/dice/payload"
)

var oKResponse []byte = []byte("+OK\r\n")

type DiceServer struct {
	port string
}

func NewDiceServer(port string) *DiceServer {
	return &DiceServer{
		port: port,
	}
}

func (srv *DiceServer) Start() error {
	ln, err := net.Listen("tcp", srv.port)
	if err != nil {
		logServerErr(err)
		return err
	}
	defer ln.Close()
	logServerInfo("Started listening on port: %s", srv.port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			logServerErr(err)
			return err
		}
		logServerInfo("a new connection arrived at: %s", conn.RemoteAddr().String())

		go srv.handleClient(conn)
	}
}

// TODO: Handle Graceful Exit by client
func (srv *DiceServer) handleClient(conn net.Conn) error {
	defer conn.Close()

	db := make(map[string][]byte)
	buff := make([]byte, 4096)
	reader := bufio.NewReader(conn)
	for {
		bytesRead, err := reader.Read(buff)
		if err != nil {
			if errors.Is(err, io.EOF) {
				logServerInfo("client disconnected gracefully.")
				return nil
			}
			logServerErr(err)
			return err
		}

		if bytesRead == 0 {
			break
		}

		input, _, err := payload.Parse(buff[0:bytesRead]) // TODO: seperate client from server errors
		if err != nil {
			diceio.SendError(conn, err.Error())
			continue
		}
		// input must be and array of bulk strings
		inputx, ok := input.(*payload.Arrays)
		if !ok {
			diceio.SendError(conn, payload.ErrInvalidPayload.Error())
			continue
		}

		if len(inputx.Value) < 1 {
			diceio.RespondWithOk(conn)
			continue
		}

		rawCmd := string(inputx.Value[0])

		if strings.EqualFold(rawCmd, "ping") {
			message := elementAt(inputx.Value, 1)
			if _, err := command.Ping(conn, message); err != nil {
				logServerErr(err)
			}
		} else if strings.EqualFold(rawCmd, "echo") {
			message := elementAt(inputx.Value, 1)
			if _, err := command.Echo(conn, message); err != nil {
				logServerErr(err)
			}
		} else if strings.EqualFold(rawCmd, "set") {
			key := elementAt(inputx.Value, 1)
			value := elementAt(inputx.Value, 2)
			if _, err := command.Set(conn, db, key, value); err != nil {
				logServerErr(err)
			}
		} else if strings.EqualFold(rawCmd, "get") {
			key := elementAt(inputx.Value, 1)
			if _, err := command.Get(conn, db, key); err != nil {
				logServerErr(err)
			}
		} else if strings.EqualFold(rawCmd, "command") {
			if _, err := command.Command(conn); err != nil {
				logServerErr(err)
			}
		} else {
			diceio.SendError(conn, "unknown command '%s'", rawCmd)
		}
	}
	return nil
}

func logServerErr(err error) {
	fmt.Fprintf(os.Stderr, "[SERVER] %v\tERROR: %v\n", time.Now().Format(time.DateTime), err.Error())
}

func logServerInfo(msg string, args ...any) {
	fmt.Fprintf(os.Stdout, "[SERVER] %v\t%v\n", time.Now().Format(time.DateTime), fmt.Sprintf(msg, args...))
}
