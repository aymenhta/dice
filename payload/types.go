package payload

import (
	"fmt"
	"strings"
)

type PayloadKind uint8

type IPayload interface {
	Kind() PayloadKind
}

type SimpleStrings struct {
	Value []byte
}

func (x *SimpleStrings) Kind() PayloadKind {
	return PAYLOAD_SIMPLE_STRINGS
}

func (x *SimpleStrings) String() string {
	return fmt.Sprintf("SimpleString(%v)", string(x.Value))
}

type Integers struct {
	Value []byte
}

func (x *Integers) Kind() PayloadKind {
	return PAYLOAD_INTEGERS
}

func (x *Integers) String() string {
	return fmt.Sprintf("Integers(%v)", string(x.Value))
}

type Errors struct {
	Value []byte
}

func (x *Errors) Kind() PayloadKind {
	return PAYLOAD_ERRORS
}

func (x *Errors) String() string {
	return fmt.Sprintf("Errors(%v)", string(x.Value))
}

type BulkStrings struct {
	Value  []byte
	IsNull bool
}

func (x *BulkStrings) Kind() PayloadKind {
	return PAYLOAD_BULK_STRINGS
}

func (x *BulkStrings) String() string {
	fmtVal := "nil"
	if x.Value != nil {
		fmtVal = string(x.Value)
	}
	return fmt.Sprintf("BulkStrings(%v)", fmtVal)
}

type Arrays struct {
	Value  [][]byte
	IsNull bool
}

func (x *Arrays) Kind() PayloadKind {
	return PAYLOAD_ARRAYS
}
func (x *Arrays) String() string {
	fmtVal := "nil"
	count := 0
	if x.Value != nil {
		count = len(x.Value)
		sb := strings.Builder{}
		for i, v := range x.Value {
			sb.WriteString(string(v))
			if i < len(x.Value)-1 {
				sb.WriteString(", ")
			}
		}
		fmtVal = sb.String()
	}

	return fmt.Sprintf("Arrays [%v items] (%v)", count, fmtVal)
}
