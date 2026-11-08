package main

import (
	"log"
	"net"

	"aymenhta.com/dice/internal/commands"
	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
	"aymenhta.com/dice/internal/utils"
)

func main() {
	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	log.Println("Server started listening on port 8888")

	store := store.NewStore()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleClient(conn, store)
	}
}

func handleClient(conn net.Conn, store *store.Store) {
	defer conn.Close()

	manager := commands.NewManager(conn)
	defer manager.Close()

	log.Printf("new client has arrived %+v\n", conn.RemoteAddr())

	if err := manager.SetOptions(resp.RespReadWriterOpts{
		MaxMultiBulkLength: utils.Pointer(512 * 1024 * 1024),
		MaxBulkLength:      utils.Pointer(65 * 1024 * 1024),
		MaxBufferSize:      utils.Pointer(4096),
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

		if err := manager.Dispatch(store, args); err != nil {
			manager.CloseWithError(err)
			log.Printf("closed connection from %s during write: %v\n", conn.RemoteAddr(), err)
			return
		}
	}
}
