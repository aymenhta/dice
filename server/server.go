package server

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"
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

		payload := buff[0:bytesRead]
		fmt.Println("received: ", string(payload))

		conn.Write(oKResponse)
	}
	return nil
}

func logServerErr(err error) {
	fmt.Fprintf(os.Stderr, "[SERVER] %v\tERROR: %v\n", time.Now().Format(time.DateTime), err.Error())
}

func logServerInfo(msg string, args ...any) {
	fmt.Fprintf(os.Stdout, "[SERVER] %v\t%v\n", time.Now().Format(time.DateTime), fmt.Sprintf(msg, args...))
}
