package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"aymenhta.com/dice/cmd/server/commands"
	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/resp"
	"aymenhta.com/dice/internal/store"
)

type DiceServer struct {
	address  string
	nWorker  int
	clientID atomic.Uint64
	store    *store.Store
}

func newDiceServer(address string, nworker int) *DiceServer {
	return &DiceServer{
		address: address,
		store:   store.NewStore(),
		nWorker: nworker,
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
		go srv.handleClient(conn, -1)
	}
}

func (srv *DiceServer) ListenAndServeFromWorker(lc net.ListenConfig, workerPID int) error {
	l, err := lc.Listen(context.Background(), "tcp", srv.address)
	if err != nil {
		return fmt.Errorf("Worker [PID %d] failed to listen: %v", workerPID, err)
	}
	defer l.Close()

	fmt.Printf("Worker [PID %d] listening on %s\n", workerPID, srv.address)

	// Setup worker context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		l.Close()
		cancel()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil // Normal exit on shutdown
			default:
				log.Printf("Worker [PID %d] accept error: %v", workerPID, err)
				continue
			}
		}
		go srv.handleClient(conn, workerPID)
	}
}

func (srv *DiceServer) handleClient(conn net.Conn, workerPID int) {
	defer conn.Close()

	manager := commands.NewManager(conn, srv.clientID.Add(1))
	defer manager.Close()

	if workerPID != -1 {
		fmt.Printf("🔌 [Worker PID %d] Handled connection from %s\n", workerPID, conn.RemoteAddr().String())
	} else {
		log.Printf("new client has arrived %+v\n", conn.RemoteAddr())
	}

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
