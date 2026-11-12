package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"aymenhta.com/dice/internal/dsa"
	"aymenhta.com/dice/internal/resp/doublebuffer"
	"aymenhta.com/dice/internal/resp/static"
	"aymenhta.com/dice/internal/resp/types"
	"aymenhta.com/dice/internal/utils"
)

const ReaderBufferSize = 65536
const WriterBufferSize = 65536

type RespReadWriter struct {
	rd   *bufio.Reader
	wr   *doublebuffer.DoubleBuffer
	rerr error
	werr error

	// buffers
	writeBuf     []byte   // holds temporary data used to merge write calls
	appendIntBuf []byte   // holds temporary data used to append integers
	argsBuffer   []byte   // holds all the arguments back to back
	argsRefs     [][]byte // holds slice references to the arguments in argsBuffer
	argsBounds   []int    // holds the start pos and len of each argument in argsBuffer

	// limits
	maxMultiBulkLength int
	maxBulkLength      int
	maxBufferSize      int

	resp2compat bool
}

type RespReadWriterOpts struct {
	// MaxMultiBulkLength sets the maximum number of elements in a multi-bulk command request.
	MaxMultiBulkLength dsa.Option[int]

	// MaxBulkLength sets the maximum length of a bulk string in bytes.
	MaxBulkLength dsa.Option[int]

	// MaxBufferSize sets the maximum size of the buffer used to read full commands from the client.
	MaxBufferSize dsa.Option[int]

	// RESP2Compat sets the RESP2 compatibility mode of the incoming connection.
	// This mode is useful when talking with a RESP2 client that does not support the new RESP3 types.
	// In this mode, Write functions will convert RESP3 types to RESP2 types where possible.
	RESP2Compat dsa.Option[bool]
}

// NewClientHandler returns a new wrapped incoming connection with sane default parameters set.
func NewRespReadWriter(rw io.ReadWriter) *RespReadWriter {
	s := &RespReadWriter{
		rd:                 bufio.NewReaderSize(rw, ReaderBufferSize),
		wr:                 doublebuffer.NewWriterSize(rw, WriterBufferSize),
		maxMultiBulkLength: MaxMultiBulkLength,
		maxBulkLength:      MaxBulkLength,
		maxBufferSize:      MaxMultiBulkLength * MaxBulkLength,
		appendIntBuf:       make([]byte, 0, 20),
	}
	return s
}

// ResetReader starts reading commands from the newly passed reader. Mostly used for testing purposes.
func (x *RespReadWriter) ResetReader(r io.Reader) {
	x.rd.Reset(r)
	x.rerr = nil
}

// Reset starts reading and writing commands from the newly passed reader and writer. Mostly used for testing purposes.
func (x *RespReadWriter) Reset(rw io.ReadWriter) {
	x.rd.Reset(rw)
	x.wr.Reset(rw)
	x.rerr = nil
	x.werr = nil
}

// SetOptions sets the options for the incoming connection.
func (x *RespReadWriter) SetOptions(opts RespReadWriterOpts) error {
	if opts.MaxMultiBulkLength.IsSome() {
		x.maxMultiBulkLength = opts.MaxMultiBulkLength.OrEmpty()
	}
	if opts.MaxBulkLength.IsSome() {
		x.maxBulkLength = opts.MaxBulkLength.OrEmpty()
	}
	if opts.MaxBufferSize.IsSome() {
		x.maxBufferSize = opts.MaxBufferSize.OrEmpty()
	}
	if opts.RESP2Compat.IsSome() {
		x.resp2compat = opts.RESP2Compat.OrEmpty()
	}
	return nil
}

func (x *RespReadWriter) MaxMultiBulkLength() int {
	return x.maxMultiBulkLength
}

func (x *RespReadWriter) MaxBulkLength() int {
	return x.maxBulkLength
}

func (x *RespReadWriter) MaxBufferSize() int {
	return x.maxBufferSize
}

func (x *RespReadWriter) RESP2Compat() bool {
	return x.resp2compat
}

// SetRESP2Compat sets the RESP2 compatibility mode of the incoming connection.
// This mode is useful when talking with a RESP2 client that does not support the new RESP3 types.
// In this mode, Write functions will convert RESP3 types to RESP2 types where possible.
func (x *RespReadWriter) SetRESP2Compat(v bool) {
	x.resp2compat = v
}

func (x *RespReadWriter) Werr() error {
	return x.werr

}

// Next reads the next command from the incoming connection and returns it as a slice of byte slices.
// The returned slice is only valid until the next call to Next as it is reused for each command.
// If in doubt, copy the slice and every byte slice it contains to a newly allocated slice and byte slices.
func (x *RespReadWriter) Next() ([][]byte, error) {
	if err := x.rerr; err != nil {
		return nil, err
	}
	for {
		if args, err := x.next(); err != nil {
			x.rerr = err
			return args, err
		} else if len(args) > 0 {
			return args, err
		}
	}
}

func (x *RespReadWriter) collectFragmentsSize(delim byte, maxLen int) (full []byte, err error) {
	var frag []byte
	fullLen := 0
	fragLen := 0
	for {
		var e error
		frag, e = x.rd.ReadSlice(delim)
		fragLen = len(frag)

		// if the full + fragment buffer has grown too large, return an error
		if fullLen+fragLen > maxLen {
			err = bufio.ErrBufferFull
			break
		}

		// if read returns no error, we have a full line, return it
		if e == nil {
			break
		}

		// if read returns an error other than bufio.ErrBufferFull, return the error
		if !errors.Is(e, bufio.ErrBufferFull) {
			err = e
			break
		}

		// copy the fragment to the full buffer so we can reuse the fragment buffer
		full = append(full, frag...)
		fullLen += fragLen
	}

	if fullLen <= 0 {
		return frag, err
	}
	if fragLen > 0 {
		full = append(full, frag...)
	}
	return full, err
}

func (x *RespReadWriter) readLine() ([]byte, error) {
	b, err := x.collectFragmentsSize('\n', x.maxBufferSize)
	if err != nil {
		return b, err
	}
	l := len(b)
	if l > 1 && b[l-2] == '\r' {
		return b[:l-2], nil
	}
	return b[:l-1], nil
}

func (x *RespReadWriter) readShortLine() ([]byte, error) {
	b, err := x.collectFragmentsSize('\n', x.rd.Size())
	if err != nil {
		return b, err
	}
	l := len(b)
	if l > 1 && b[l-2] == '\r' {
		return b[:l-2], nil
	}
	return b[:l-1], nil
}

func hexDigitToInt(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	c |= 'x' - 'X'
	if c >= 'a' && c <= 'f' {
		return 10 + int(c-'a')
	}
	return -1
}

var readArgsSpaceChars = [256]uint8{' ': 1, '\r': 1, '\n': 2, '\v': 3, '\f': 3, '\t': 1, '\x00': 1}

func (x *RespReadWriter) readArgs() ([][]byte, error) {
	argsRefs := x.argsRefs
	argsBounds := x.argsBounds[:0]
	argsBuffer := x.argsBuffer[:0]
	defer func() {
		x.argsRefs = argsRefs
		x.argsBounds = argsBounds
		x.argsBuffer = argsBuffer
	}()
	argc := 0
	barrier := 0

	indq := false // set to true if we are in "double quotes"
	insq := false // set to true if we are in 'single quotes'

	for {
		for {
			c, err := x.rd.ReadByte()
			if err != nil {
				return nil, err
			}
			switch readArgsSpaceChars[c] {
			case 1, 3: // spaces
				continue
			case 2: // newline (\n)
				if insq || indq {
					return nil, utils.ErrProtoUnbalancedQuotes
				}
				argsRefs = utils.Expand(argsRefs, argc)
				for i := 0; i < argc; i++ {
					argsRefs[i] = argsBuffer[argsBounds[i*2] : argsBounds[i*2]+argsBounds[i*2+1]]
				}
				return argsRefs[:argc], nil
			}
			x.rd.UnreadByte()
			break
		}

	inner:
		for {
			c, err := x.rd.ReadByte()
			if err != nil {
				break // we handle read errors at the start of the outer loop
			}

			if indq {
				if c == '\n' {
					return nil, utils.ErrProtoUnbalancedQuotes
				}
				if c == '\\' {
					peek, err := x.rd.Peek(3)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) >= 3 && peek[0] == 'x' {
						i0 := hexDigitToInt(peek[1])
						i1 := hexDigitToInt(peek[2])
						if i0 >= 0 && i1 >= 0 {
							c = byte(i0<<4 | i1)
							if len(argsBuffer)+1 > x.maxBufferSize {
								return nil, bufio.ErrBufferFull
							}
							argsBuffer = append(argsBuffer, c)
							x.rd.Discard(3)
							continue
						}
					}
					if len(peek) >= 1 {
						switch peek[0] {
						case 'n':
							c = '\n'
						case 'r':
							c = '\r'
						case 't':
							c = '\t'
						case 'b':
							c = '\b'
						case 'a':
							c = '\a'
						default:
							c = peek[0]
						}
						if len(argsBuffer)+1 > x.maxBufferSize {
							return nil, bufio.ErrBufferFull
						}
						argsBuffer = append(argsBuffer, c)
						x.rd.Discard(1)
						continue
					}
				}
				if c == '"' {
					peek, err := x.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) > 0 && readArgsSpaceChars[peek[0]] == 0 {
						return nil, utils.ErrProtoUnbalancedQuotes
					}
					indq = false
					break
				}
				if len(argsBuffer)+1 > x.maxBufferSize {
					return nil, bufio.ErrBufferFull
				}
				argsBuffer = append(argsBuffer, c)
				continue
			}
			if insq {
				if c == '\n' {
					return nil, utils.ErrProtoUnbalancedQuotes
				}
				if c == '\\' {
					peek, err := x.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) >= 1 && peek[0] == '\'' {
						c = '\''
						if len(argsBuffer)+1 > x.maxBufferSize {
							return nil, bufio.ErrBufferFull
						}
						argsBuffer = append(argsBuffer, c)
						x.rd.Discard(1)
						continue
					}
				}
				if c == '\'' {
					peek, err := x.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) > 0 && readArgsSpaceChars[peek[0]] == 0 {
						return nil, utils.ErrProtoUnbalancedQuotes
					}
					insq = false
					break
				}
				if len(argsBuffer)+1 > x.maxBufferSize {
					return nil, bufio.ErrBufferFull
				}
				argsBuffer = append(argsBuffer, c)
				continue
			}
			switch c {
			case '"':
				indq = true
			case '\'':
				insq = true
			default:
				switch readArgsSpaceChars[c] {
				case 1, 2:
					x.rd.UnreadByte()
					break inner
				}
				if len(argsBuffer)+1 > x.maxBufferSize {
					return nil, bufio.ErrBufferFull
				}
				argsBuffer = append(argsBuffer, c)
			}
		}
		size := len(argsBuffer) - barrier
		if size > x.maxBulkLength {
			return nil, utils.ErrProtoInvalidBulkLength
		}
		argc++
		if argc > x.maxMultiBulkLength {
			return nil, utils.ErrProtoInvalidMultiBulkLength
		}
		argsBounds = append(argsBounds, barrier, size)
		barrier = len(argsBuffer)
	}
}

func (x *RespReadWriter) next() ([][]byte, error) {
	t, err := x.rd.Peek(1)
	if err != nil {
		return nil, err
	}
	if len(t) == 0 {
		return nil, nil
	}
	switch t[0] {
	case types.RespArray:
	default:
		return x.readArgs()
	}
	line, err := x.readShortLine()
	if err != nil {
		return nil, err
	}
	if len(line) <= 1 {
		return nil, nil
	}
	n32, err := utils.ParseInt32(line[1:])
	n := int(n32)
	if err != nil || n > x.maxMultiBulkLength {
		return nil, utils.ErrProtoInvalidMultiBulkLength
	}
	if n <= 0 {
		return nil, nil
	}

	// fmt.Println("=====================================")
	// fmt.Println("array size is:", n)

	argsRefs := utils.Expand(x.argsRefs, n)
	argsBounds := utils.Expand(x.argsBounds, int(n)*2)
	argsBuffer := x.argsBuffer
	defer func() {
		x.argsRefs = argsRefs
		x.argsBounds = argsBounds
		x.argsBuffer = argsBuffer
	}()

	p := 0
	for i := 0; i < n; i++ {
		line, err := x.readShortLine()
		if err != nil {
			return nil, err
		}
		if len(line) == 0 || line[0] != types.RespBulkStr {
			return nil, utils.ErrProtoExpectedString
		}
		l32, err := utils.ParseUInt32(line[1:])
		l := int(l32)
		if err != nil || l < 0 || l > x.maxBulkLength {
			return nil, utils.ErrProtoInvalidBulkLength
		}
		if p+l+2 > x.maxBufferSize {
			return nil, bufio.ErrBufferFull
		}
		argsBuffer = utils.Expand(argsBuffer, p+l+2)
		readBufferChunk := argsBuffer[p : p+l+2]
		argsBounds[i*2] = p
		argsBounds[i*2+1] = l
		if _, err := io.ReadFull(x.rd, readBufferChunk); err != nil {
			return nil, err
		}
		// fmt.Printf("read data (%v) is: %v--\n", len(string(readBufferChunk)), string(readBufferChunk))
		p += l
		// fmt.Println("string size is:", l)
	}
	for i := 0; i < n; i++ {
		argsRefs[i] = argsBuffer[argsBounds[i*2] : argsBounds[i*2]+argsBounds[i*2+1]]
	}
	return argsRefs[:n], nil
}

// Close gracefully closes the incoming connection after flushing any pending writes.
func (x *RespReadWriter) Close() error {
	return x.wr.Close()
}

// CloseWithError closes the incoming connection after writing an error response.
// This is a convenience function that combines WriteError and Close.
func (x *RespReadWriter) CloseWithError(err error) error {
	if err != nil {
		x.WriteError(err)
	}
	return x.Close()
}

func (x *RespReadWriter) write(b []byte) error {
	if x.werr != nil {
		return x.werr
	}
	if len(b) == 0 {
		return nil
	}
	_, err := x.wr.Write(b)
	if err != nil {
		x.werr = err
	}
	return err
}

var crlfLen = len(types.CRLF)

func (x *RespReadWriter) writeWithType(typeByte byte, head []byte, body []byte) error {
	headLen := len(head)
	bodyLen := len(body)
	replySize := 1 + headLen + crlfLen
	if body != nil {
		replySize += bodyLen + crlfLen
	}
	replyBuf := utils.Expand(x.writeBuf, replySize)
	replyBuf[0] = typeByte
	p := 1
	if headLen > 0 {
		copy(replyBuf[p:], utils.SanitizeSimpleString(head))
		p += headLen
	}
	copy(replyBuf[p:], types.CRLF)
	p += crlfLen
	if body != nil {
		copy(replyBuf[p:], body)
		p += bodyLen
		copy(replyBuf[p:], types.CRLF)
		p += crlfLen
	}
	x.writeBuf = replyBuf
	return x.write(replyBuf[:p])
}

func (x *RespReadWriter) writeWithPrefix(prefix []byte, head []byte, body []byte) error {
	prefixLen := len(prefix)
	headLen := len(head)
	bodyLen := len(body)
	replySize := prefixLen + headLen + crlfLen
	if body != nil {
		replySize += bodyLen + crlfLen
	}
	replyBuf := utils.Expand(x.writeBuf, replySize)
	copy(replyBuf, prefix)
	p := prefixLen
	if headLen > 0 {
		copy(replyBuf[p:], utils.SanitizeSimpleString(head))
		p += headLen
	}
	copy(replyBuf[p:], types.CRLF)
	p += crlfLen
	if body != nil {
		copy(replyBuf[p:], body)
		p += bodyLen
		copy(replyBuf[p:], types.CRLF)
		p += crlfLen
	}
	x.writeBuf = replyBuf
	return x.write(replyBuf[:p])
}

// WriteSimpleString writes a [Simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-string-reply).
func (x *RespReadWriter) WriteStatusBytes(v []byte) error {
	return x.writeWithType(types.RespSimpleStr, v, nil)
}

// WriteSimpleString writes a [Simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-string-reply).
func (x *RespReadWriter) WriteSimpleString(v string) error {
	return x.WriteStatusBytes(utils.StrB(v))
}

// WriteError writes a sanitized [Simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#error-reply).
func (x *RespReadWriter) WriteError(e error) error {
	return x.writeWithPrefix(static.RespPrefixedErrorBytes, utils.StrB(e.Error()), nil)
}

func (x *RespReadWriter) WriteWrongTypeError() error {
	return x.writeWithPrefix(
		static.RespPrefixedWrongTypeErrBytes,
		utils.StrB(utils.ErrWrongTypeErr.Error()),
		nil,
	)
}

// WriteOK is a convenience method for calling WriteStatusBytes with "OK".
func (x *RespReadWriter) WriteOK() error {
	return x.writeWithPrefix(static.RespPrefixedOKBytes, nil, nil)
}

// WriteBytes writes a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply).
func (x *RespReadWriter) WriteBytes(v []byte) error {
	if v == nil {
		v = static.NullBytes
	}
	return x.writeWithType(types.RespBulkStr, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), v)
}

// WriteString writes a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply).
func (x *RespReadWriter) WriteString(v string) error {
	return x.WriteBytes(utils.StrB(v))
}

// WriteInt writes an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply).
func (x *RespReadWriter) WriteInt(v int) error {
	return x.WriteInt64(int64(v))
}

// WriteInt64 writes an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply).
func (x *RespReadWriter) WriteInt64(v int64) error {
	return x.writeWithType(types.RespInt, strconv.AppendInt(x.appendIntBuf[:0], v, 10), nil)
}

// WriteExplicitNullString writes a [Null Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-reply).
func (x *RespReadWriter) WriteExplicitNullString() error {
	return x.writeWithPrefix(static.RespPrefixedNullStringBytes, nil, nil)
}

// WriteExplicitNullArray writes a [Null Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-array-reply).
func (x *RespReadWriter) WriteExplicitNullArray() error {
	return x.writeWithPrefix(static.RespPrefixedNullArrayBytes, nil, nil)
}

// WriteNullString writes a [Null](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-reply) for RESP3 connections or a [Null Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-reply) for RESP2 connections.
func (x *RespReadWriter) WriteNullString() error {
	if x.resp2compat {
		return x.WriteExplicitNullString()
	}
	return x.writeWithType(types.RespNil, nil, nil)
}

// WriteNullArray writes a [Null](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-reply) for RESP3 connections or a [Null Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-array-reply) for RESP2 connections.
func (x *RespReadWriter) WriteNullArray() error {
	if x.resp2compat {
		return x.WriteExplicitNullArray()
	}
	return x.writeWithType(types.RespNil, nil, nil)
}

// WriteFloat writes a [Double](https://redis.io/docs/latest/develop/reference/protocol-spec/#double-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) containing the string representation of the float for RESP2 connections.
func (x *RespReadWriter) WriteFloat(v float64) error {
	if x.resp2compat {
		return x.WriteBytes(strconv.AppendFloat(nil, v, 'g', -1, 64))
	}
	return x.writeWithType(types.RespFloat, strconv.AppendFloat(nil, v, 'g', -1, 64), nil)
}

// WriteBool writes a [Boolean](https://redis.io/docs/latest/develop/reference/protocol-spec/#boolean-reply) for RESP3 connections or an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply) of `0` or `1` for RESP2 connections.
func (x *RespReadWriter) WriteBool(v bool) error {
	if x.resp2compat {
		if v {
			return x.WriteInt(1)
		} else {
			return x.WriteInt(0)
		}
	}
	if v {
		return x.writeWithPrefix(static.RespPrefixedBoolTrueBytes, nil, nil)
	}
	return x.writeWithPrefix(static.RespPrefixedBoolFalseBytes, nil, nil)
}

// WriteBlobError writes a [Bulk error](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-error-reply) for RESP3 connections or a sanitized [Simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#error-reply) for RESP2 connections.
func (x *RespReadWriter) WriteBlobError(e error) error {
	if x.resp2compat {
		return x.WriteError(e)
	}
	v := utils.StrB(e.Error())
	return x.writeWithType(types.RespBlobError, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), v)
}

// WriteBlobString writes a [Verbatim string](https://redis.io/docs/latest/develop/reference/protocol-spec/#verbatim-string-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) for RESP2 connections.
// The verbatim string needs to contain the data encoding part and the data itself. Example: `txt:Arbitrary text data`. The data encoding part is stripped when sending the data as Bulk string to RESP2 clients.
func (x *RespReadWriter) WriteVerbatimBytes(v []byte) error {
	vLen := len(v)
	if x.resp2compat {
		if vLen >= 4 {
			return x.WriteBytes(v[4:])
		}
		return x.WriteBytes(v)
	}
	if v == nil {
		v = static.NullBytes
	}
	return x.writeWithType(types.RespVerbatim, strconv.AppendInt(x.appendIntBuf[:0], int64(vLen), 10), v)
}

// WriteBlobString writes a [Verbatim string](https://redis.io/docs/latest/develop/reference/protocol-spec/#verbatim-string-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) for RESP2 connections.
// The verbatim string needs to contain the data encoding part and the data itself. Example: `txt:Arbitrary text data`. The data encoding part is stripped when sending the data as Bulk string to RESP2 clients.
func (x *RespReadWriter) WriteVerbatimString(v string) error {
	return x.WriteVerbatimBytes(utils.StrB(v))
}

// WriteBigInt writes a [Big number](https://redis.io/docs/latest/develop/reference/protocol-spec/#big-number-reply) for RESP3 connections or an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply) for RESP2 connections.
// If v cannot be represented in an int64, the result is undefined when sending to a RESP2 client.
func (x *RespReadWriter) WriteBigInt(v big.Int) error {
	if x.resp2compat {
		return x.WriteInt64(v.Int64())
	}
	return x.writeWithType(types.RespBigInt, v.Append(nil, 10), nil)
}

// WriteArrayHeader writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header.
func (x *RespReadWriter) WriteArrayHeader(l int) error {
	return x.writeWithType(types.RespArray, strconv.AppendInt(x.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteArrayBytes is a convenience method to write an array of Bulk strings.
func (x *RespReadWriter) WriteArrayBytes(v [][]byte) error {
	if err := x.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

func (x *RespReadWriter) WriteArrayInt64(v []int64) error {
	if err := x.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteInt64(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteArrayString is a convenience method to write an array of Bulk strings.
func (x *RespReadWriter) WriteArrayString(v []string) error {
	if err := x.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteArray writes an array of any values that can be converted to a RESP type.
func (x *RespReadWriter) WriteArray(v []any) error {
	if err := x.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

func (x *RespReadWriter) WriteEmptyArray() error {
	if err := x.WriteArrayHeader(0); err != nil {
		return err
	}
	return nil
}

// WriteMapHeader writes a [Map](https://redis.io/docs/latest/develop/reference/protocol-spec/#map-reply) header with the specified length. For RESP2 connections, this writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header with twice the specified length.
func (x *RespReadWriter) WriteMapHeader(l int) error {
	if x.resp2compat {
		return x.WriteArrayHeader(l * 2)
	}
	return x.writeWithType(types.RespMap, strconv.AppendInt(x.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteMapBytes is a convenience method to write a map of Bulk strings.
func (x *RespReadWriter) WriteMapBytes(v map[string][]byte) error {
	if err := x.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteMapString is a convenience method to write a map of Bulk strings.
func (x *RespReadWriter) WriteMapString(v map[string]string) error {
	if err := x.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteMap writes a map of any values that can be converted to a RESP type.
func (x *RespReadWriter) WriteMap(v map[string]any) error {
	if err := x.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetHeader writes a [Set](https://redis.io/docs/latest/develop/reference/protocol-spec/#set-reply) header with the specified length. For RESP2 connections, this writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header with twice the specified length.
func (x *RespReadWriter) WriteSetHeader(l int) error {
	if x.resp2compat {
		return x.WriteArrayHeader(l)
	}
	return x.writeWithType(types.RespSet, strconv.AppendInt(x.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteSetBytes is a convenience method to write a set of Bulk strings.
func (x *RespReadWriter) WriteSetBytes(v [][]byte) error {
	if err := x.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetString is a convenience method to write a set of Bulk strings.
func (x *RespReadWriter) WriteSetString(v []string) error {
	if err := x.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSet writes a set of any values that can be converted to a RESP type.
func (x *RespReadWriter) WriteSet(v []any) error {
	if err := x.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttrBytes writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (x *RespReadWriter) WriteAttrBytes(v map[string][]byte) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespAttr, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttrString writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (x *RespReadWriter) WriteAttrString(v map[string]string) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespAttr, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttr writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (x *RespReadWriter) WriteAttr(v map[string]any) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespAttr, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := x.WriteString(k); err != nil {
			return err
		}
		if err := x.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (x *RespReadWriter) WritePushBytes(v [][]byte) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespPush, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (x *RespReadWriter) WritePushString(v []string) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespPush, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (x *RespReadWriter) WritePush(v []any) error {
	if x.resp2compat {
		return nil
	}
	if err := x.writeWithType(types.RespPush, strconv.AppendInt(x.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := x.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// Write writes any value that can be converted to a RESP type.
func (x *RespReadWriter) Write(v any) error {
	switch v := v.(type) {
	case []byte:
		return x.WriteBytes(v)
	case string:
		return x.WriteString(v)
	case int:
		return x.WriteInt(v)
	case int32:
		return x.WriteInt(int(v))
	case int64:
		return x.WriteInt64(v)
	case float64:
		return x.WriteFloat(v)
	case bool:
		return x.WriteBool(v)
	case error:
		return x.WriteError(v)
	case nil:
		return x.WriteNullString()
	case []any:
		return x.WriteArray(v)
	case map[string]any:
		return x.WriteMap(v)
	case big.Int:
		return x.WriteBigInt(v)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

// WriteRaw writes raw RESP data to the connection.
// You can also use this function to check for any previous write errors by writing a nil or empty value.
func (x *RespReadWriter) WriteRaw(v []byte) error {
	return x.write(v)
}
