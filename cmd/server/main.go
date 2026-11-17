package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/aymenhta/dice/internal/utils"
)

func main() {
	opts := parseRunOptions()

	switch runtime.GOOS {
	case "linux":
		nWorkers := utils.MaxInt(opts.nworkers, 1)
		runWithWorkers(opts, nWorkers)
	default:
		run(opts.addr)
	}
}

func run(addr string) {
	srv := newDiceServer(addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func runWithWorkers(cfg RunOptions, nWorkers int) {
	if !cfg.isWorker {
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
		srv := newDiceServerForWorker(cfg.addr, nWorkers)
		if err := srv.ListenAndServeFromWorker(workerPID); err != nil {
			log.Fatal(err)
		}
	}
}
