package resp

import (
	"bytes"
	"io"
	"strconv"
)

func SerializeTo(w io.Writer, payload Frame) (int, error) {
	b, err := Serialize(payload)
	if err != nil {
		return 0, err
	}
	return w.Write(b)
}

func Serialize(payload Frame) ([]byte, error) {
	switch payload.Kind() {

	case PAYLOAD_SIMPLE_STRINGS:
		item, ok := payload.(*SimpleString)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeSimpleStrings(item)

	case PAYLOAD_INTEGERS:
		item, ok := payload.(*Integer)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeIntegers(item)

	case PAYLOAD_ERRORS:
		item, ok := payload.(*Error)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeErrors(item)

	case PAYLOAD_BULK_STRINGS:
		item, ok := payload.(*BulkString)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeBulkStrings(item)

	case PAYLOAD_ARRAYS:
		item, ok := payload.(*Array)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeArrays(item)

	default:
		return nil, ErrInvalidPayload
	}
}

func serializeSimpleStrings(x *SimpleString) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("+")
	buff.Write(x.Value)
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}

func serializeIntegers(x *Integer) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString(":")
	buff.Write(x.Value)
	buff.WriteString(SEP)

	return buff.Bytes(), nil
}

func serializeErrors(x *Error) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("-")
	buff.Write(x.Value)
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}

func serializeBulkStrings(x *BulkString) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("$")
	if x.IsNull {
		buff.WriteString("-1")
	} else {
		buff.WriteString(strconv.Itoa(len(x.Value)))
		buff.WriteString(SEP)
		buff.Write(x.Value)
	}
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}

// Current implementation convert its values into an array of byte strings
// this doesn't support nested arrays, arrays of integers, ...etc
//
// POSSIBLE FIX: use the option API
func serializeArrays(x *Array) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("*")
	if x.IsNull {
		buff.WriteString("-1")
		buff.WriteString(SEP)
		return buff.Bytes(), nil
	}

	if len(x.Value) == 0 {
		buff.WriteString("0")
		buff.WriteString(SEP)
		return buff.Bytes(), nil
	}

	buff.WriteString(strconv.Itoa(len(x.Value)))
	buff.WriteString(SEP)

	for _, b := range x.Value {
		// Serialize them as byte strings
		bs := &BulkString{Value: b, IsNull: b == nil}
		bData, err := serializeBulkStrings(bs)
		if err != nil {
			return nil, err
		}
		buff.Write(bData)
	}
	return buff.Bytes(), nil
}
