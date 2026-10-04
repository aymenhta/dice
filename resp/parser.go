package resp

import (
	"errors"
	"fmt"
	"strconv"
)

func Parse(input []byte) (Frame, []byte, error) {
	inputLen := len(input)
	if inputLen < 1 {
		return nil, nil, fmt.Errorf("something went wrong")
	}

	if !(input[inputLen-1] == '\n' && input[inputLen-2] == '\r') {
		return nil, input, errors.New("invalid payload")
	}

	kind, err := frameKindFromByte(input[0])
	if err != nil {
		return nil, input, err
	}

	switch kind {
	case PAYLOAD_SIMPLE_STRINGS:
		return parseSimpleString(input)
	case PAYLOAD_INTEGERS:
		return parseIntegers(input)
	case PAYLOAD_ERRORS:
		return parseErrors(input)
	case PAYLOAD_BULK_STRINGS:
		return parseBulkStrings(input)
	case PAYLOAD_ARRAYS:
		return parseArrays(input)
		// panic("not implemented!")
	default:
		return nil, nil, err
	}

}

func parseSimpleString(input []byte) (Frame, []byte, error) {
	// skip kind
	pos := 1

	// read until you get \r\n
	start := pos
	inputLen := len(input)
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)
	for pos < inputLen && !(currentByte == '\r' && nextByte == '\n') {
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	// Skip \r \n
	restIdx := pos + 2

	// TODO: check if value contains \n or \r

	return &SimpleString{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

func parseIntegers(input []byte) (Frame, []byte, error) {
	// skip kind
	pos := 1
	// read until you get \r\n
	start := pos
	inputLen := len(input)
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)
	for pos < inputLen && !(currentByte == '\r' && nextByte == '\n') {
		if pos == 1 && currentByte == '-' {
			goto post_loop
		} else if !(isDigit(currentByte)) {
			return nil, input, errors.New("unvalid integer")
		}

	post_loop:
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	// skip \r \n
	restIdx := pos + 2

	return &Integer{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

func parseErrors(input []byte) (Frame, []byte, error) {
	// skip kind
	pos := 1

	// read until you get \r\n
	start := pos
	inputLen := len(input)
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)
	for pos < inputLen && !(currentByte == '\r' && nextByte == '\n') {
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	// skip \r \n
	restIdx := pos + 2

	return &Error{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

// $<length>\r\n<data>\r\n
func parseBulkStrings(input []byte) (Frame, []byte, error) {
	inputLen := len(input)

	// skip kind
	pos := 1

	// read length
	lengthStart := pos
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)

	if currentByte == '-' && nextByte == '1' {
		return &BulkString{Value: nil, IsNull: true}, input[pos+4:], nil
	}

	for !(currentByte == '\r' && nextByte == '\n') && isDigit(currentByte) {
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	expectedLen, err := strconv.Atoi(string(input[lengthStart:pos]))
	if err != nil {
		return nil, input, fmt.Errorf("Error during conversion: %v", err)
	}

	// SKIP \r\n
	pos += 2

	if expectedLen == 0 {
		if !(currentByte == '\r' && nextByte == '\n') {
			return nil, input, errors.New("invalid payload")
		}

		return &BulkString{Value: []byte(""), IsNull: false}, input[pos+2:], nil
	}

	// read until you get \r\n
	start := pos
	currentByte = safeIndex(input, pos)
	nextByte = safeIndex(input, pos+1)
	for pos < inputLen && !(currentByte == '\r' && nextByte == '\n') {
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	content := input[start:pos]
	if len(content) != int(expectedLen) {
		return nil, input, fmt.Errorf("length of bulk string content doesn't match, expected %v but got %v", expectedLen, len(content))
	}

	// Skip \r \n
	restIdx := pos + 2
	if restIdx > inputLen {
		restIdx = inputLen
	}

	return &BulkString{
		Value:  content,
		IsNull: false,
	}, input[restIdx:], nil
}

// *<number-of-elements>\r\n<element-1>...<element-n>
func parseArrays(input []byte) (Frame, []byte, error) {
	// skip kind
	pos := 1

	// read length
	lengthStart := pos
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)

	if currentByte == '-' && nextByte == '1' {
		return &Array{Value: nil, ItemsKinds: nil, IsNull: true}, input[pos+4:], nil
	}

	for !(currentByte == '\r' && nextByte == '\n') && isDigit(currentByte) {
		pos++
		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	expectedLen, err := strconv.Atoi(string(input[lengthStart:pos]))
	if err != nil {
		return nil, input, fmt.Errorf("Error during conversion: %v", err)
	}

	if expectedLen < 0 {
		return nil, input, errors.New("invalid payload")
	}

	// SKIP \r\n
	pos += 2

	var container [][]byte
	var itemsKinds []FrameKind
	for range expectedLen {
		kind, err := frameKindFromByte(safeIndex(input, pos))
		if err != nil {
			return nil, input, err
		}

		if kind == PAYLOAD_SIMPLE_STRINGS {
			item, restOfInput, err := parseSimpleString(input[pos:])
			if err != nil {
				return nil, input, err
			}

			if x, ok := item.(*SimpleString); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
				itemsKinds = append(itemsKinds, item.Kind())
			}
			input = restOfInput
		} else if kind == PAYLOAD_INTEGERS {
			item, restOfInput, err := parseIntegers(input[pos:])
			if err != nil {
				return nil, input, err

			}
			if x, ok := item.(*Integer); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
				itemsKinds = append(itemsKinds, item.Kind())
			}
			input = restOfInput
		} else if kind == PAYLOAD_ERRORS {
			item, restOfInput, err := parseErrors(input[pos:])
			if err != nil {
				return nil, input, err

			}
			if x, ok := item.(*Error); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
				itemsKinds = append(itemsKinds, item.Kind())
			}
			input = restOfInput
		} else if kind == PAYLOAD_BULK_STRINGS {
			item, restOfInput, err := parseBulkStrings(input[pos:])
			if err != nil {
				return nil, input, err
			}
			if x, ok := item.(*BulkString); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
				itemsKinds = append(itemsKinds, item.Kind())
			}
			input = restOfInput
		} else if kind == PAYLOAD_ARRAYS {
			panic("not implemented!!")
		}
		pos = 0
	}

	return &Array{
		Value:      container,
		ItemsKinds: itemsKinds,
		IsNull:     false,
	}, input, nil
}

func debugCurrByte(b byte) {
	bfmt := string(b)
	if b == '\r' {
		bfmt = "\\r"
	} else if b == '\n' {
		bfmt = "\\n"
	}
	fmt.Printf("Current byte is: %v\n", bfmt)
}

func debugBytes(bs []byte) {
	fmt.Print("input is: ")
	for _, b := range bs {
		bfmt := string(b)
		if b == '\r' {
			bfmt = "\\r"
		} else if b == '\n' {
			bfmt = "\\n"
		}
		fmt.Printf("%v", bfmt)
	}

	fmt.Printf("\n")
}
