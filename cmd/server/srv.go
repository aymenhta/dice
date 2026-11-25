package main

import (
	"log"
	"net"
	"sync/atomic"

	"github.com/aymenhta/dice/cmd/server/commands"
	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/resp"
	"github.com/aymenhta/dice/internal/store"
)

type DiceServer struct {
	address  string
	clientID atomic.Uint64
	store    *store.Store
}

func newDiceServer(address string) *DiceServer {
	return &DiceServer{
		address: address,
		store:   store.NewStore(),
	}
}

func (srv *DiceServer) ListenAndServe() error {
	l, err := net.Listen("tcp", srv.address)
	if err != nil {
		return err
	}
	defer l.Close()

	log.Println("Server started listening on:", srv.address)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go srv.handleClient(conn)
	}
}

func (srv *DiceServer) handleClient(conn net.Conn) {
	defer conn.Close()

	manager := commands.NewManager(conn, srv.clientID.Add(1))
	defer manager.Close()
	log.Printf("new client has arrived %+v\n", conn.RemoteAddr().String())

	if err := manager.SetOptions(resp.RespReadWriterOpts{
		MaxMultiBulkLength: dsa.Some(512 * 1024 * 1024),
		MaxBulkLength:      dsa.Some(65 * 1024 * 1024),
		MaxBufferSize:      dsa.Some(4096),
		RESP2Compat:        dsa.Some(true),
	}); err != nil {
		manager.CloseWithError(err)
	}

	for {
		args, err := manager.Next()
		if err != nil {
			manager.CloseWithError(err)
			log.Printf("closed connection from %s, during read: %v\n", conn.RemoteAddr(), err)
			return
		}

		if err := manager.Dispatch(srv.store, args); err != nil {
			manager.CloseWithError(err)
			log.Printf("closed connection from %s during write: %v\n", conn.RemoteAddr(), err)
			return
		}
	}
}
