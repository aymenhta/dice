// if the error is a client error, then send it to them
// otherwise return the error to the caller

package command

import (
	"io"

	"aymenhta.com/dice/diceio"
	"aymenhta.com/dice/payload"
)

func Ping(w io.Writer, message []byte) (int, error) {
	var response payload.IPayload

	if len(message) < 1 {
		response = &payload.SimpleStrings{Value: []byte("PONG")}
	} else {
		response = &payload.BulkStrings{Value: message}
	}

	return payload.SerializeTo(w, response)
}

func Echo(w io.Writer, message []byte) (int, error) {
	var response payload.IPayload

	if len(message) < 1 {
		return diceio.SendError(w, "ERR wrong number of arguments for command")
	}

	response = &payload.BulkStrings{Value: message, IsNull: false}
	return payload.SerializeTo(w, response)
}

func Command(w io.Writer) (int, error) {
	return diceio.RespondWithOk(w)
}

func Set(w io.Writer, db map[string][]byte, key, value []byte) (int, error) {
	db[string(key)] = value
	return diceio.RespondWithOk(w)
}

func Get(w io.Writer, db map[string][]byte, key []byte) (int, error) {
	var response payload.IPayload
	value, ok := db[string(key)]
	if !ok {
		return diceio.RespondWithNullBulkStr(w)
	}

	response = &payload.BulkStrings{Value: value, IsNull: false}
	return payload.SerializeTo(w, response)
}
