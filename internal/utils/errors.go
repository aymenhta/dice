package utils

import "errors"

var (
	ErrProtoUnbalancedQuotes       = errors.New("Protocol error: unbalanced quotes in request")
	ErrProtoInvalidMultiBulkLength = errors.New("Protocol error: invalid multibulk length")
	ErrProtoInvalidBulkLength      = errors.New("Protocol error: invalid bulk length")
	ErrProtoExpectedString         = errors.New("Protocol error: expected '$'")
	ErrSyntaxError                 = errors.New("syntax error")
	ErrBadDigest                   = errors.New("digest must be exactly 16 hexadecimal characters")
	ErrNotInt                      = errors.New("value is not an integer or out of range")
	ErrIncrDecrOverflow            = errors.New("increment or decrement would overflow")
	ErrWrongTypeErr                = errors.New("Operation against a key holding the wrong kind of value")
	ErrNoSuchKey                   = errors.New("no such key")
	ErrIndexOutOfRange             = errors.New("index out of range")
)
