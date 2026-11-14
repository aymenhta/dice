package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"aymenhta.com/dice/internal/utils"
)

type Config struct {
	isWorker bool
	addr     string
	nworkers int
}

func main() {
	var cfg Config
	flag.BoolVar(&cfg.isWorker, "worker", false, "run as a worker process")
	flag.StringVar(&cfg.addr, "addr", "0.0.0.0:8888", "listen addres")
	flag.IntVar(&cfg.nworkers, "nw", runtime.NumCPU()-1, "number of workers")
	flag.Parse()
	nWorkers := utils.MaxInt(cfg.nworkers, 1)

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
		srv := newDiceServer(cfg.addr, nWorkers)
		if err := srv.ListenAndServeFromWorker(workerPID); err != nil {
			log.Fatal(err)
		}
	}

}
