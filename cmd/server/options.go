package main

import (
	"flag"
)

type RunOptions struct {
	addr string
}

func parseRunOptions() RunOptions {
	var cfg RunOptions
	flag.StringVar(&cfg.addr, "addr", "0.0.0.0:8888", "listen addres")
	flag.Parse()

	return cfg
}
