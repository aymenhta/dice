package main

import (
	"log"
	"net"

	"aymenhta.com/dice/resp"
	"aymenhta.com/dice/store"
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

	handler := resp.NewClientHandler(conn)
	defer handler.Close()

	log.Printf("new client has arrived %+v\n", conn.RemoteAddr())

	if err := handler.SetOptions(resp.ClientHandlerOpts{
		MaxMultiBulkLength: resp.Pointer(1024),
		MaxBulkLength:      resp.Pointer(65 * 1024 * 1024),
		MaxBufferSize:      resp.Pointer(4096),
	}); err != nil {
		handler.CloseWithError(err)
	}

	for {
		args, err := handler.Next()
		if err != nil {
			handler.CloseWithError(err)
			log.Printf("closed connection from %s, during read: %v\n", conn.RemoteAddr(), err)
			return
		}

		if handler.Dispatch(store, args); err != nil {
			handler.CloseWithError(err)
			log.Printf("closed connection from %s during write: %v\n", conn.RemoteAddr(), err)
			return
		}
	}
}
