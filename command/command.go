package command

import (
	"aymenhta.com/dice/payload"
)

func Ping(message []byte) ([]byte, error) {
	var response payload.IPayload

	if len(message) < 1 {
		response = &payload.SimpleStrings{Value: []byte("PONG")}
	} else {
		response = &payload.BulkStrings{Value: message}
	}

	responseAsBytes, err := payload.Serialize(response)
	if err != nil {
		return nil, err
	}

	return responseAsBytes, nil
}
