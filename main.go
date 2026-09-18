package main

import (
	"log"

	"aymenhta.com/dice/server"
)

func main() {
	dice := server.NewDiceServer(":8888")
	if err := dice.Start(); err != nil {
		log.Fatal(err)
	}
}
