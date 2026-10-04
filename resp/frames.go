package resp

import (
	"bytes"
	"fmt"
	"strings"
)

type SimpleString struct {
	Value []byte
}

func NewOkSimpleString() Frame {
	return &SimpleString{Value: []byte("OK")}
}

func (x *SimpleString) Kind() FrameKind {
	return PAYLOAD_SIMPLE_STRINGS
}

func (x *SimpleString) String() string {
	return fmt.Sprintf("SimpleString(%v)", string(x.Value))
}

type Integer struct {
	Value []byte
}

func (x *Integer) Kind() FrameKind {
	return PAYLOAD_INTEGERS
}

func (x *Integer) String() string {
	var xfmt string
	if bytes.Compare(x.Value, []byte("0")) == 0 {
		xfmt = "0"
	} else {
		xfmt = string(x.Value)
	}
	return fmt.Sprintf("Integers(%v)", xfmt)
}

type Error struct {
	Value []byte
}

func (x *Error) Kind() FrameKind {
	return PAYLOAD_ERRORS
}

func (x *Error) String() string {
	return fmt.Sprintf("Errors(%v)", string(x.Value))
}

func NewError(msg string) Frame {
	return &Error{Value: []byte(msg)}
}

type BulkString struct {
	Value  []byte
	IsNull bool
}

func NewNullBulkString() *BulkString {
	return &BulkString{Value: []byte("$-1\r\n"), IsNull: true}
}

func NewBulkString(value []byte) *BulkString {
	return &BulkString{Value: value, IsNull: false}
}

func (x *BulkString) Kind() FrameKind {
	return PAYLOAD_BULK_STRINGS
}

func (x *BulkString) String() string {
	fmtVal := "nil"
	if x.Value != nil {
		fmtVal = string(x.Value)
	}
	return fmt.Sprintf("BulkStrings(%v)", fmtVal)
}

type Array struct {
	Value      [][]byte
	ItemsKinds []FrameKind
	IsNull     bool
}

func (x *Array) Kind() FrameKind {
	return PAYLOAD_ARRAYS
}
func (x *Array) String() string {
	fmtVal := "nil"
	count := 0
	if x.Value != nil {
		count = len(x.Value)
		sb := strings.Builder{}
		for i, v := range x.Value {
			sb.Write(v)
			if i < len(x.Value)-1 {
				sb.WriteString(", ")
			}
		}
		fmtVal = sb.String()
	}

	return fmt.Sprintf("Arrays [%v items] (%v)", count, fmtVal)
}
