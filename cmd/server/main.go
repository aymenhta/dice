package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"aymenhta.com/dice/internal/utils"
	"golang.org/x/sys/unix"
)

var workerFlag = flag.Bool("worker", false, "run as a worker process")

func main() {
	flag.Parse()
	addr := "0.0.0.0:8888"
	nWorkers := utils.MaxInt(runtime.NumCPU()-1, 1)

	if !*workerFlag {
		// ==================================
		// LEADER PROCESS
		// ==================================
		leaderPID := os.Getpid()
		fmt.Printf("Leader Process [PID %d] starting...\n", leaderPID)
		fmt.Printf("Spawning %d worker processes...\n", nWorkers)

		workers := make([]*exec.Cmd, nWorkers)
		for i := range nWorkers {
			// Re-execute the current binary with the --worker flag
			cmd := exec.Command(os.Args[0], "--worker")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin

			if err := cmd.Start(); err != nil {
				log.Fatalf("failed to start worker %d: %v", i, err)
			}
			workers[i] = cmd
		}

		// Trap Signals for graceful shutdown of the entire pool
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("Leader shutting down, signaling workers...")
		for _, w := range workers {
			if w.Process != nil {
				_ = w.Process.Signal(syscall.SIGTERM)
			}
		}

		// wait for all workers to exit
		for _, w := range workers {
			_ = w.Wait()
		}
		fmt.Println("Leader exited cleanly.")
	} else {
		// ===================================
		// WORKER PROCESS
		// ===================================
		workerPID := os.Getpid()

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

		srv := newDiceServer(addr, nWorkers)
		if err := srv.ListenAndServeFromWorker(lc, workerPID); err != nil {
			log.Fatal(err)
		}
	}

}
