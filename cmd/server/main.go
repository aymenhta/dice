package main

import (
	"log"
)

func main() {
	opts := parseRunOptions()

	srv := newDiceServer(opts.addr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
