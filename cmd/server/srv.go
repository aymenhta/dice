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

	"github.com/aymenhta/dice/cmd/server/commands"
	"github.com/aymenhta/dice/internal/dsa"
	"github.com/aymenhta/dice/internal/resp"
	"github.com/aymenhta/dice/internal/store"
	"golang.org/x/sys/unix"
)

type DiceServer struct {
	address  string
	nWorker  int
	clientID atomic.Uint64
	store    *store.Store
}

func newDiceServerForWorker(address string, nworker int) *DiceServer {
	return &DiceServer{
		address: address,
		store:   store.NewStore(),
		nWorker: nworker,
	}
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
		go srv.handleClient(conn, -1)
	}
}

func (srv *DiceServer) ListenAndServeFromWorker(workerPID int) error {
	lc := net.ListenConfig{
		Control: func(network string, address string, c syscall.RawConn) error {
			var innerErr error
			err := c.Control(func(fd uintptr) {
				// Enable SO_REUSEADDR
				innerErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if innerErr != nil {
					return
				}
				// Enable SO_REUSEPORT so multiple workers can bind the same port
				innerErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
			if err != nil {
				return err
			}

			return innerErr
		},
	}
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
		fmt.Printf("[Worker PID %d] Handled connection from %s\n", workerPID, conn.RemoteAddr().String())
	} else {
		log.Printf("new client has arrived %+v\n", conn.RemoteAddr().String())
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
