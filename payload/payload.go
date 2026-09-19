package payload

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPayload = errors.New("ERR invalid payload.")
)

const SEP = "\r\n"

const (
	PAYLOAD_SIMPLE_STRINGS PayloadKind = 1
	PAYLOAD_INTEGERS       PayloadKind = 2
	PAYLOAD_ERRORS         PayloadKind = 3
	PAYLOAD_BULK_STRINGS   PayloadKind = 4
	PAYLOAD_ARRAYS         PayloadKind = 5
)

func PayloadKindFromByte(x byte) (PayloadKind, error) {
	switch x {
	case '+':
		return PAYLOAD_SIMPLE_STRINGS, nil
	case ':':
		return PAYLOAD_INTEGERS, nil
	case '-':
		return PAYLOAD_ERRORS, nil
	case '$':
		return PAYLOAD_BULK_STRINGS, nil
	case '*':
		return PAYLOAD_ARRAYS, nil
	default:
		xfmt := string(x)
		if x == '\r' {
			xfmt = "\\r"
		} else if x == '\n' {
			xfmt = "\\n"
		}
		return 0, fmt.Errorf("[ERROR]: unkown payload identifier '%v'", xfmt)
	}
}
