package server

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"aymenhta.com/dice/command"
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

	buff := make([]byte, 4096)
	reader := bufio.NewReader(conn)
	for {
		bytesRead, err := reader.Read(buff)
		if err != nil {
			logServerErr(err)
			return err
		}

		if bytesRead == 0 {
			break
		}

		input, _, err := payload.Parse(buff[0:bytesRead])
		if err != nil {
			msg := "-ERR " + err.Error() + "\r\n" // TODO: only if the error is a client error
			conn.Write([]byte(msg))
			continue
		}
		// input must be and array of bulk strings
		inputx, ok := input.(*payload.Arrays)
		if !ok {
			conn.Write([]byte("-ERR invalid paylod\r\n"))
			continue
		}

		if len(inputx.Value) < 1 {
			conn.Write(oKResponse)
			continue
		}

		rawCmd := string(inputx.Value[0])

		if strings.EqualFold(rawCmd, "ping") {
			message, ok := elementAt(inputx.Value, 1)
			if !ok {
				message = nil
			}
			response, err := command.Ping(message)
			if err != nil {
				msg := "-ERR " + err.Error() + "\r\n" // TODO: only if the error is a client error
				conn.Write([]byte(msg))
				continue
			} else {
				conn.Write(response)
			}

			continue
		} else {
			conn.Write([]byte("-ERR invalid command (TODO: insert command name)\r\n"))
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
