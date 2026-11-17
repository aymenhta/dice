package main

import (
	"flag"
	"runtime"
)

type RunOptions struct {
	isWorker bool
	addr     string
	nworkers int
}

func parseRunOptions() RunOptions {
	var cfg RunOptions
	flag.BoolVar(&cfg.isWorker, "worker", false, "run as a worker process")
	flag.StringVar(&cfg.addr, "addr", "0.0.0.0:8888", "listen addres")
	flag.IntVar(&cfg.nworkers, "nw", runtime.NumCPU()-1, "number of workers")
	flag.Parse()

	return cfg
}
