package payload

import (
	"errors"
	"fmt"
	"strconv"
)

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

func ParsePayload(input []byte) (IPayload, []byte, error) {
	inputLen := len(input)
	if inputLen < 1 {
		return nil, nil, fmt.Errorf("something went wrong")
	}

	if !(input[inputLen-1] == '\n' && input[inputLen-2] == '\r') {
		return nil, input, errors.New("invalid payload")
	}

	kind, err := PayloadKindFromByte(input[0])
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

func parseSimpleString(input []byte) (IPayload, []byte, error) {
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

	return &SimpleStrings{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

func parseIntegers(input []byte) (IPayload, []byte, error) {
	// skip kind
	pos := 1

	// read until you get \r\n
	start := pos
	inputLen := len(input)
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)
	for pos < inputLen && !(currentByte == '\r' && nextByte == '\n') {
		if !(isDigit(currentByte)) {
			return nil, input, errors.New("unvalid integer")
		}
		pos++

		currentByte = safeIndex(input, pos)
		nextByte = safeIndex(input, pos+1)
	}

	// skip \r \n
	restIdx := pos + 2

	return &Integers{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

func parseErrors(input []byte) (IPayload, []byte, error) {
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

	return &Errors{
		Value: input[start:pos],
	}, input[restIdx:], nil
}

// $<length>\r\n<data>\r\n
func parseBulkStrings(input []byte) (IPayload, []byte, error) {
	inputLen := len(input)

	// skip kind
	pos := 1

	// read length
	lengthStart := pos
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)

	if currentByte == '-' && nextByte == '1' {
		return &BulkStrings{Value: nil, IsNull: true}, input[pos+4:], nil
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

		return &BulkStrings{Value: []byte(""), IsNull: false}, input[pos+2:], nil
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

	return &BulkStrings{
		Value:  content,
		IsNull: false,
	}, input[restIdx:], nil
}

// *<number-of-elements>\r\n<element-1>...<element-n>
func parseArrays(input []byte) (IPayload, []byte, error) {
	// skip kind
	pos := 1

	// read length
	lengthStart := pos
	currentByte := safeIndex(input, pos)
	nextByte := safeIndex(input, pos+1)

	if currentByte == '-' && nextByte == '1' {
		return &Arrays{Value: nil, IsNull: true}, input[pos+3:], nil
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

	for range expectedLen {
		kind, err := PayloadKindFromByte(safeIndex(input, pos))
		if err != nil {
			return nil, input, err
		}

		if kind == PAYLOAD_SIMPLE_STRINGS {
			item, restOfInput, err := parseSimpleString(input[pos:])
			if err != nil {
				return nil, input, err
			}

			if x, ok := item.(*SimpleStrings); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
			}
			input = restOfInput
		} else if kind == PAYLOAD_INTEGERS {
			item, restOfInput, err := parseIntegers(input[pos:])
			if err != nil {
				return nil, input, err

			}
			if x, ok := item.(*Integers); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
			}
			input = restOfInput
		} else if kind == PAYLOAD_ERRORS {
			item, restOfInput, err := parseErrors(input[pos:])
			if err != nil {
				return nil, input, err

			}
			if x, ok := item.(*Errors); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
			}
			input = restOfInput
		} else if kind == PAYLOAD_BULK_STRINGS {
			item, restOfInput, err := parseBulkStrings(input[pos:])
			if err != nil {
				return nil, input, err

			}
			if x, ok := item.(*BulkStrings); ok {
				cloneditem := make([]byte, len(x.Value))
				copy(cloneditem, x.Value)
				container = append(container, cloneditem)
			}
			input = restOfInput
		} else if kind == PAYLOAD_ARRAYS {
			panic("not implemented!!")
		}
		pos = 0
	}

	return &Arrays{
		Value:  container,
		IsNull: false,
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
