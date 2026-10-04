package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"aymenhta.com/dice/resp"
	"aymenhta.com/dice/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	dice := server.NewDiceServer(":8888", logger)
	if err := dice.Start(); err != nil {
		log.Fatal(err)
	}
	// testPayloadParsing()
}

func testPayloadParsing() {
	cases := []string{
		"$-1\r\n",
		"*-1\r\n",
		"*0\r\n",
		":0\r\n",
		":15\r\n",
		":-66\r\n",
		"*1\r\n$4\r\nping\r\n",
		"*2\r\n$4\r\necho\r\n$11\r\nhello world\r\n",
		"*2\r\n$3\r\nget\r\n$3\r\nkey\r\n",
		"*3\r\n$3\r\nset\r\n$3\r\nkey\r\n$5\r\nvalue\r\n",
		"+OK\r\n",
		"-Error message\r\n",
		"$0\r\n\r\n",
		"$6\r\ndotnet\r\n",
		"+hello world\r\n",
	}

	for _, tcase := range cases {
		item, _, err := resp.Parse([]byte(tcase))
		if err != nil {
			log.Fatal(err)
		}

		serialization, err := resp.Serialize(item)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Parsing result is: %+v\nSerialization result == input: %v\n", item, string(serialization) == tcase)
		fmt.Printf("Serialization is: %v\n", string(serialization))
		fmt.Println("-------------------------------------------------------------")
	}
}
