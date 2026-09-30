package resp

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPayload = errors.New("ERR invalid payload.")
)

const SEP = "\r\n"

type FrameKind uint8

const (
	PAYLOAD_SIMPLE_STRINGS FrameKind = iota
	PAYLOAD_INTEGERS
	PAYLOAD_ERRORS
	PAYLOAD_BULK_STRINGS
	PAYLOAD_ARRAYS
)

type Frame interface {
	Kind() FrameKind
}

func frameKindFromByte(x byte) (FrameKind, error) {
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
