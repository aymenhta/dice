package payload

import (
	"bytes"
	"strconv"
)

func Serialize(payload IPayload) ([]byte, error) {
	switch payload.Kind() {

	case PAYLOAD_SIMPLE_STRINGS:
		item, ok := payload.(*SimpleStrings)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeSimpleStrings(item)

	case PAYLOAD_INTEGERS:
		item, ok := payload.(*Integers)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeIntegers(item)

	case PAYLOAD_ERRORS:
		item, ok := payload.(*Errors)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeErrors(item)

	case PAYLOAD_BULK_STRINGS:
		item, ok := payload.(*BulkStrings)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeBulkStrings(item)

	case PAYLOAD_ARRAYS:
		item, ok := payload.(*Arrays)
		if !ok {
			return nil, ErrInvalidPayload
		}

		return serializeArrays(item)

	default:
		return nil, ErrInvalidPayload
	}
}

func serializeSimpleStrings(x *SimpleStrings) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("+")
	buff.Write(x.Value)
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}
func serializeIntegers(x *Integers) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString(":")
	buff.Write(x.Value)
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}
func serializeErrors(x *Errors) ([]byte, error) {
	var buff bytes.Buffer
	buff.WriteString("-")
	buff.Write(x.Value)
	buff.WriteString(SEP)
	return buff.Bytes(), nil
}
func serializeBulkStrings(x *BulkStrings) ([]byte, error) {
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
func serializeArrays(x *Arrays) ([]byte, error) {
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
		bs := &BulkStrings{Value: b, IsNull: b == nil}
		bData, err := serializeBulkStrings(bs)
		if err != nil {
			return nil, err
		}
		buff.Write(bData)
	}
	return buff.Bytes(), nil
}
