package resp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"

	"aymenhta.com/dice/resp/doublebuffer"
	"aymenhta.com/dice/resp/static"
	"aymenhta.com/dice/resp/types"
	"aymenhta.com/dice/store"
)

const ReaderBufferSize = 65536
const WriterBufferSize = 65536

type ClientHandler struct {
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

	// store
	currentDB int

	resp2compat bool
}

type ClientHandlerOpts struct {
	// MaxMultiBulkLength sets the maximum number of elements in a multi-bulk command request.
	MaxMultiBulkLength *int

	// MaxBulkLength sets the maximum length of a bulk string in bytes.
	MaxBulkLength *int

	// MaxBufferSize sets the maximum size of the buffer used to read full commands from the client.
	MaxBufferSize *int

	// RESP2Compat sets the RESP2 compatibility mode of the incoming connection.
	// This mode is useful when talking with a RESP2 client that does not support the new RESP3 types.
	// In this mode, Write functions will convert RESP3 types to RESP2 types where possible.
	RESP2Compat *bool
}

// NewClientHandler returns a new wrapped incoming connection with sane default parameters set.
func NewClientHandler(rw io.ReadWriter) *ClientHandler {
	s := &ClientHandler{
		rd:                 bufio.NewReaderSize(rw, ReaderBufferSize),
		wr:                 doublebuffer.NewWriterSize(rw, WriterBufferSize),
		maxMultiBulkLength: MaxMultiBulkLength,
		maxBulkLength:      MaxBulkLength,
		maxBufferSize:      MaxMultiBulkLength * MaxBulkLength,
		appendIntBuf:       make([]byte, 0, 20),
		currentDB:          0,
	}
	return s
}

func (h *ClientHandler) Dispatch(store *store.Store, args [][]byte) error {
	cmd := bstring(bytesToLower(args[0]))
	args = args[1:]

	db := store.DB[h.currentDB]

	switch cmd {
	case "ping":
		return h.cmdPing(args)
	case "echo":
		return h.cmdEcho(args)
	case "set":
		return h.cmdSet(db, args)
	case "get":
		return h.cmdGet(db, args)
	case "del":
		return h.cmdDel(db, args)
	case "exists":
		return h.cmdExists(db, args)
	case "expire":
		return h.cmdExpire(db, args)
	case "ttl":
		return h.cmdTtl(db, args)
	case "incr":
		return h.cmdIncr(db, args)
	case "decr":
		return h.cmdDecr(db, args)
	case "incrby":
		return h.cmdIncrBy(db, args)
	case "decrby":
		return h.cmdDecrBy(db, args)
	case "command":
		return h.cmdCommand()
	case "quit":
		return h.cmdQuit()
	default:
		return h.WriteError(fmt.Errorf("unkown command '%s'", cmd))
	}
}

// ResetReader starts reading commands from the newly passed reader. Mostly used for testing purposes.
func (h *ClientHandler) ResetReader(r io.Reader) {
	h.rd.Reset(r)
	h.rerr = nil
}

// Reset starts reading and writing commands from the newly passed reader and writer. Mostly used for testing purposes.
func (h *ClientHandler) Reset(rw io.ReadWriter) {
	h.rd.Reset(rw)
	h.wr.Reset(rw)
	h.rerr = nil
	h.werr = nil
}

// SetOptions sets the options for the incoming connection.
func (h *ClientHandler) SetOptions(opts ClientHandlerOpts) error {
	if opts.MaxMultiBulkLength != nil {
		h.maxMultiBulkLength = *opts.MaxMultiBulkLength
	}
	if opts.MaxBulkLength != nil {
		h.maxBulkLength = *opts.MaxBulkLength
	}
	if opts.MaxBufferSize != nil {
		h.maxBufferSize = *opts.MaxBufferSize
	}
	if opts.RESP2Compat != nil {
		h.resp2compat = *opts.RESP2Compat
	}
	return nil
}

// SetRESP2Compat sets the RESP2 compatibility mode of the incoming connection.
// This mode is useful when talking with a RESP2 client that does not support the new RESP3 types.
// In this mode, Write functions will convert RESP3 types to RESP2 types where possible.
func (h *ClientHandler) SetRESP2Compat(v bool) {
	h.resp2compat = v
}

func (h *ClientHandler) Werr() error {
	return h.werr

}

// Next reads the next command from the incoming connection and returns it as a slice of byte slices.
// The returned slice is only valid until the next call to Next as it is reused for each command.
// If in doubt, copy the slice and every byte slice it contains to a newly allocated slice and byte slices.
func (h *ClientHandler) Next() ([][]byte, error) {
	if err := h.rerr; err != nil {
		return nil, err
	}
	for {
		if args, err := h.next(); err != nil {
			h.rerr = err
			return args, err
		} else if len(args) > 0 {
			return args, err
		}
	}
}

func (h *ClientHandler) collectFragmentsSize(delim byte, maxLen int) (full []byte, err error) {
	var frag []byte
	fullLen := 0
	fragLen := 0
	for {
		var e error
		frag, e = h.rd.ReadSlice(delim)
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

func (h *ClientHandler) readLine() ([]byte, error) {
	b, err := h.collectFragmentsSize('\n', h.maxBufferSize)
	if err != nil {
		return b, err
	}
	l := len(b)
	if l > 1 && b[l-2] == '\r' {
		return b[:l-2], nil
	}
	return b[:l-1], nil
}

func (h *ClientHandler) readShortLine() ([]byte, error) {
	b, err := h.collectFragmentsSize('\n', h.rd.Size())
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

func (h *ClientHandler) readArgs() ([][]byte, error) {
	argsRefs := h.argsRefs
	argsBounds := h.argsBounds[:0]
	argsBuffer := h.argsBuffer[:0]
	defer func() {
		h.argsRefs = argsRefs
		h.argsBounds = argsBounds
		h.argsBuffer = argsBuffer
	}()
	argc := 0
	barrier := 0

	indq := false // set to true if we are in "double quotes"
	insq := false // set to true if we are in 'single quotes'

	for {
		for {
			c, err := h.rd.ReadByte()
			if err != nil {
				return nil, err
			}
			switch readArgsSpaceChars[c] {
			case 1, 3: // spaces
				continue
			case 2: // newline (\n)
				if insq || indq {
					return nil, ErrProtoUnbalancedQuotes
				}
				argsRefs = Expand(argsRefs, argc)
				for i := 0; i < argc; i++ {
					argsRefs[i] = argsBuffer[argsBounds[i*2] : argsBounds[i*2]+argsBounds[i*2+1]]
				}
				return argsRefs[:argc], nil
			}
			h.rd.UnreadByte()
			break
		}

	inner:
		for {
			c, err := h.rd.ReadByte()
			if err != nil {
				break // we handle read errors at the start of the outer loop
			}

			if indq {
				if c == '\n' {
					return nil, ErrProtoUnbalancedQuotes
				}
				if c == '\\' {
					peek, err := h.rd.Peek(3)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) >= 3 && peek[0] == 'x' {
						i0 := hexDigitToInt(peek[1])
						i1 := hexDigitToInt(peek[2])
						if i0 >= 0 && i1 >= 0 {
							c = byte(i0<<4 | i1)
							if len(argsBuffer)+1 > h.maxBufferSize {
								return nil, bufio.ErrBufferFull
							}
							argsBuffer = append(argsBuffer, c)
							h.rd.Discard(3)
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
						if len(argsBuffer)+1 > h.maxBufferSize {
							return nil, bufio.ErrBufferFull
						}
						argsBuffer = append(argsBuffer, c)
						h.rd.Discard(1)
						continue
					}
				}
				if c == '"' {
					peek, err := h.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) > 0 && readArgsSpaceChars[peek[0]] == 0 {
						return nil, ErrProtoUnbalancedQuotes
					}
					indq = false
					break
				}
				if len(argsBuffer)+1 > h.maxBufferSize {
					return nil, bufio.ErrBufferFull
				}
				argsBuffer = append(argsBuffer, c)
				continue
			}
			if insq {
				if c == '\n' {
					return nil, ErrProtoUnbalancedQuotes
				}
				if c == '\\' {
					peek, err := h.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) >= 1 && peek[0] == '\'' {
						c = '\''
						if len(argsBuffer)+1 > h.maxBufferSize {
							return nil, bufio.ErrBufferFull
						}
						argsBuffer = append(argsBuffer, c)
						h.rd.Discard(1)
						continue
					}
				}
				if c == '\'' {
					peek, err := h.rd.Peek(1)
					if err != nil && !errors.Is(err, io.EOF) {
						return nil, err
					}
					if len(peek) > 0 && readArgsSpaceChars[peek[0]] == 0 {
						return nil, ErrProtoUnbalancedQuotes
					}
					insq = false
					break
				}
				if len(argsBuffer)+1 > h.maxBufferSize {
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
					h.rd.UnreadByte()
					break inner
				}
				if len(argsBuffer)+1 > h.maxBufferSize {
					return nil, bufio.ErrBufferFull
				}
				argsBuffer = append(argsBuffer, c)
			}
		}
		size := len(argsBuffer) - barrier
		if size > h.maxBulkLength {
			return nil, ErrProtoInvalidBulkLength
		}
		argc++
		if argc > h.maxMultiBulkLength {
			return nil, ErrProtoInvalidMultiBulkLength
		}
		argsBounds = append(argsBounds, barrier, size)
		barrier = len(argsBuffer)
	}
}

func (h *ClientHandler) next() ([][]byte, error) {
	t, err := h.rd.Peek(1)
	if err != nil {
		return nil, err
	}
	if len(t) == 0 {
		return nil, nil
	}
	switch t[0] {
	case types.RespArray:
	default:
		return h.readArgs()
	}
	line, err := h.readShortLine()
	if err != nil {
		return nil, err
	}
	if len(line) <= 1 {
		return nil, nil
	}
	n32, err := ParseInt32(line[1:])
	n := int(n32)
	if err != nil || n > h.maxMultiBulkLength {
		return nil, ErrProtoInvalidMultiBulkLength
	}
	if n <= 0 {
		return nil, nil
	}

	// fmt.Println("=====================================")
	// fmt.Println("array size is:", n)

	argsRefs := Expand(h.argsRefs, n)
	argsBounds := Expand(h.argsBounds, int(n)*2)
	argsBuffer := h.argsBuffer
	defer func() {
		h.argsRefs = argsRefs
		h.argsBounds = argsBounds
		h.argsBuffer = argsBuffer
	}()

	p := 0
	for i := 0; i < n; i++ {
		line, err := h.readShortLine()
		if err != nil {
			return nil, err
		}
		if len(line) == 0 || line[0] != types.RespBulkStr {
			return nil, ErrProtoExpectedString
		}
		l32, err := ParseUInt32(line[1:])
		l := int(l32)
		if err != nil || l < 0 || l > h.maxBulkLength {
			return nil, ErrProtoInvalidBulkLength
		}
		if p+l+2 > h.maxBufferSize {
			return nil, bufio.ErrBufferFull
		}
		argsBuffer = Expand(argsBuffer, p+l+2)
		readBufferChunk := argsBuffer[p : p+l+2]
		argsBounds[i*2] = p
		argsBounds[i*2+1] = l
		if _, err := io.ReadFull(h.rd, readBufferChunk); err != nil {
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
func (h *ClientHandler) Close() error {
	return h.wr.Close()
}

// CloseWithError closes the incoming connection after writing an error response.
// This is a convenience function that combines WriteError and Close.
func (h *ClientHandler) CloseWithError(err error) error {
	if err != nil {
		h.WriteError(err)
	}
	return h.Close()
}

func (h *ClientHandler) write(b []byte) error {
	if h.werr != nil {
		return h.werr
	}
	if len(b) == 0 {
		return nil
	}
	_, err := h.wr.Write(b)
	if err != nil {
		h.werr = err
	}
	return err
}

var crlfLen = len(types.CRLF)

func (h *ClientHandler) writeWithType(typeByte byte, head []byte, body []byte) error {
	headLen := len(head)
	bodyLen := len(body)
	replySize := 1 + headLen + crlfLen
	if body != nil {
		replySize += bodyLen + crlfLen
	}
	replyBuf := Expand(h.writeBuf, replySize)
	replyBuf[0] = typeByte
	p := 1
	if headLen > 0 {
		copy(replyBuf[p:], SanitizeSimpleString(head))
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
	h.writeBuf = replyBuf
	return h.write(replyBuf[:p])
}

func (h *ClientHandler) writeWithPrefix(prefix []byte, head []byte, body []byte) error {
	prefixLen := len(prefix)
	headLen := len(head)
	bodyLen := len(body)
	replySize := prefixLen + headLen + crlfLen
	if body != nil {
		replySize += bodyLen + crlfLen
	}
	replyBuf := Expand(h.writeBuf, replySize)
	copy(replyBuf, prefix)
	p := prefixLen
	if headLen > 0 {
		copy(replyBuf[p:], SanitizeSimpleString(head))
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
	h.writeBuf = replyBuf
	return h.write(replyBuf[:p])
}

// WriteSimpleString writes a [Simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-string-reply).
func (h *ClientHandler) WriteStatusBytes(v []byte) error {
	return h.writeWithType(types.RespSimpleStr, v, nil)
}

// WriteSimpleString writes a [Simple string](https://redis.io/docs/latest/develop/reference/protocol-spec/#simple-string-reply).
func (h *ClientHandler) WriteSimpleString(v string) error {
	return h.WriteStatusBytes(sbytes(v))
}

// WriteError writes a sanitized [Simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#error-reply).
func (h *ClientHandler) WriteError(e error) error {
	return h.writeWithPrefix(static.RespPrefixedErrorBytes, sbytes(e.Error()), nil)
}

func (h *ClientHandler) WriteWrongTypeError() error {
	return h.writeWithPrefix(
		static.RespPrefixedWrongTypeErrBytes,
		sbytes("Operation against a key holding the wrong kind of value"),
		nil,
	)
}

// WriteOK is a convenience method for calling WriteStatusBytes with "OK".
func (h *ClientHandler) WriteOK() error {
	return h.writeWithPrefix(static.RespPrefixedOKBytes, nil, nil)
}

// WriteBytes writes a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply).
func (h *ClientHandler) WriteBytes(v []byte) error {
	if v == nil {
		v = static.NullBytes
	}
	return h.writeWithType(types.RespBulkStr, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), v)
}

// WriteString writes a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply).
func (h *ClientHandler) WriteString(v string) error {
	return h.WriteBytes(sbytes(v))
}

// WriteInt writes an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply).
func (h *ClientHandler) WriteInt(v int) error {
	return h.WriteInt64(int64(v))
}

// WriteInt64 writes an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply).
func (h *ClientHandler) WriteInt64(v int64) error {
	return h.writeWithType(types.RespInt, strconv.AppendInt(h.appendIntBuf[:0], v, 10), nil)
}

// WriteExplicitNullString writes a [Null Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-reply).
func (h *ClientHandler) WriteExplicitNullString() error {
	return h.writeWithPrefix(static.RespPrefixedNullStringBytes, nil, nil)
}

// WriteExplicitNullArray writes a [Null Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-array-reply).
func (h *ClientHandler) WriteExplicitNullArray() error {
	return h.writeWithPrefix(static.RespPrefixedNullArrayBytes, nil, nil)
}

// WriteNullString writes a [Null](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-reply) for RESP3 connections or a [Null Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-reply) for RESP2 connections.
func (h *ClientHandler) WriteNullString() error {
	if h.resp2compat {
		return h.WriteExplicitNullString()
	}
	return h.writeWithType(types.RespNil, nil, nil)
}

// WriteNullArray writes a [Null](https://redis.io/docs/latest/develop/reference/protocol-spec/#null-reply) for RESP3 connections or a [Null Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#nil-array-reply) for RESP2 connections.
func (h *ClientHandler) WriteNullArray() error {
	if h.resp2compat {
		return h.WriteExplicitNullArray()
	}
	return h.writeWithType(types.RespNil, nil, nil)
}

// WriteFloat writes a [Double](https://redis.io/docs/latest/develop/reference/protocol-spec/#double-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) containing the string representation of the float for RESP2 connections.
func (h *ClientHandler) WriteFloat(v float64) error {
	if h.resp2compat {
		return h.WriteBytes(strconv.AppendFloat(nil, v, 'g', -1, 64))
	}
	return h.writeWithType(types.RespFloat, strconv.AppendFloat(nil, v, 'g', -1, 64), nil)
}

// WriteBool writes a [Boolean](https://redis.io/docs/latest/develop/reference/protocol-spec/#boolean-reply) for RESP3 connections or an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply) of `0` or `1` for RESP2 connections.
func (h *ClientHandler) WriteBool(v bool) error {
	if h.resp2compat {
		if v {
			return h.WriteInt(1)
		} else {
			return h.WriteInt(0)
		}
	}
	if v {
		return h.writeWithPrefix(static.RespPrefixedBoolTrueBytes, nil, nil)
	}
	return h.writeWithPrefix(static.RespPrefixedBoolFalseBytes, nil, nil)
}

// WriteBlobError writes a [Bulk error](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-error-reply) for RESP3 connections or a sanitized [Simple error](https://redis.io/docs/latest/develop/reference/protocol-spec/#error-reply) for RESP2 connections.
func (h *ClientHandler) WriteBlobError(e error) error {
	if h.resp2compat {
		return h.WriteError(e)
	}
	v := sbytes(e.Error())
	return h.writeWithType(types.RespBlobError, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), v)
}

// WriteBlobString writes a [Verbatim string](https://redis.io/docs/latest/develop/reference/protocol-spec/#verbatim-string-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) for RESP2 connections.
// The verbatim string needs to contain the data encoding part and the data itself. Example: `txt:Arbitrary text data`. The data encoding part is stripped when sending the data as Bulk string to RESP2 clients.
func (h *ClientHandler) WriteVerbatimBytes(v []byte) error {
	vLen := len(v)
	if h.resp2compat {
		if vLen >= 4 {
			return h.WriteBytes(v[4:])
		}
		return h.WriteBytes(v)
	}
	if v == nil {
		v = static.NullBytes
	}
	return h.writeWithType(types.RespVerbatim, strconv.AppendInt(h.appendIntBuf[:0], int64(vLen), 10), v)
}

// WriteBlobString writes a [Verbatim string](https://redis.io/docs/latest/develop/reference/protocol-spec/#verbatim-string-reply) for RESP3 connections or a [Bulk string](https://redis.io/docs/latest/develop/reference/protocol-spec/#bulk-string-reply) for RESP2 connections.
// The verbatim string needs to contain the data encoding part and the data itself. Example: `txt:Arbitrary text data`. The data encoding part is stripped when sending the data as Bulk string to RESP2 clients.
func (h *ClientHandler) WriteVerbatimString(v string) error {
	return h.WriteVerbatimBytes(sbytes(v))
}

// WriteBigInt writes a [Big number](https://redis.io/docs/latest/develop/reference/protocol-spec/#big-number-reply) for RESP3 connections or an [Integer](https://redis.io/docs/latest/develop/reference/protocol-spec/#integer-reply) for RESP2 connections.
// If v cannot be represented in an int64, the result is undefined when sending to a RESP2 client.
func (h *ClientHandler) WriteBigInt(v big.Int) error {
	if h.resp2compat {
		return h.WriteInt64(v.Int64())
	}
	return h.writeWithType(types.RespBigInt, v.Append(nil, 10), nil)
}

// WriteArrayHeader writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header.
func (h *ClientHandler) WriteArrayHeader(l int) error {
	return h.writeWithType(types.RespArray, strconv.AppendInt(h.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteArrayBytes is a convenience method to write an array of Bulk strings.
func (h *ClientHandler) WriteArrayBytes(v [][]byte) error {
	if err := h.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteArrayString is a convenience method to write an array of Bulk strings.
func (h *ClientHandler) WriteArrayString(v []string) error {
	if err := h.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteArray writes an array of any values that can be converted to a RESP type.
func (h *ClientHandler) WriteArray(v []any) error {
	if err := h.WriteArrayHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteMapHeader writes a [Map](https://redis.io/docs/latest/develop/reference/protocol-spec/#map-reply) header with the specified length. For RESP2 connections, this writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header with twice the specified length.
func (h *ClientHandler) WriteMapHeader(l int) error {
	if h.resp2compat {
		return h.WriteArrayHeader(l * 2)
	}
	return h.writeWithType(types.RespMap, strconv.AppendInt(h.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteMapBytes is a convenience method to write a map of Bulk strings.
func (h *ClientHandler) WriteMapBytes(v map[string][]byte) error {
	if err := h.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteMapString is a convenience method to write a map of Bulk strings.
func (h *ClientHandler) WriteMapString(v map[string]string) error {
	if err := h.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteMap writes a map of any values that can be converted to a RESP type.
func (h *ClientHandler) WriteMap(v map[string]any) error {
	if err := h.WriteMapHeader(len(v)); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetHeader writes a [Set](https://redis.io/docs/latest/develop/reference/protocol-spec/#set-reply) header with the specified length. For RESP2 connections, this writes an [Array](https://redis.io/docs/latest/develop/reference/protocol-spec/#array-reply) header with twice the specified length.
func (h *ClientHandler) WriteSetHeader(l int) error {
	if h.resp2compat {
		return h.WriteArrayHeader(l)
	}
	return h.writeWithType(types.RespSet, strconv.AppendInt(h.appendIntBuf[:0], int64(l), 10), nil)
}

// WriteSetBytes is a convenience method to write a set of Bulk strings.
func (h *ClientHandler) WriteSetBytes(v [][]byte) error {
	if err := h.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSetString is a convenience method to write a set of Bulk strings.
func (h *ClientHandler) WriteSetString(v []string) error {
	if err := h.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteSet writes a set of any values that can be converted to a RESP type.
func (h *ClientHandler) WriteSet(v []any) error {
	if err := h.WriteSetHeader(len(v)); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttrBytes writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (h *ClientHandler) WriteAttrBytes(v map[string][]byte) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespAttr, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttrString writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (h *ClientHandler) WriteAttrString(v map[string]string) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespAttr, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WriteAttr writes an [Attribute](https://github.com/antirez/RESP3/blob/master/spec.md#attribute-type) with the given data. For RESP2 connections, function calls are silently discarded and no data is written
func (h *ClientHandler) WriteAttr(v map[string]any) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespAttr, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for k, arg := range v {
		if err := h.WriteString(k); err != nil {
			return err
		}
		if err := h.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (h *ClientHandler) WritePushBytes(v [][]byte) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespPush, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteBytes(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (h *ClientHandler) WritePushString(v []string) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespPush, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.WriteString(arg); err != nil {
			return err
		}
	}
	return nil
}

// WritePushHeader writes a [Push](https://redis.io/docs/latest/develop/reference/protocol-spec/#push-event) event with the given data. For RESP2 connections, function calls are silently discarded and no data is written.
func (h *ClientHandler) WritePush(v []any) error {
	if h.resp2compat {
		return nil
	}
	if err := h.writeWithType(types.RespPush, strconv.AppendInt(h.appendIntBuf[:0], int64(len(v)), 10), nil); err != nil {
		return err
	}
	for _, arg := range v {
		if err := h.Write(arg); err != nil {
			return err
		}
	}
	return nil
}

// Write writes any value that can be converted to a RESP type.
func (h *ClientHandler) Write(v any) error {
	switch v := v.(type) {
	case []byte:
		return h.WriteBytes(v)
	case string:
		return h.WriteString(v)
	case int:
		return h.WriteInt(v)
	case int32:
		return h.WriteInt(int(v))
	case int64:
		return h.WriteInt64(v)
	case float64:
		return h.WriteFloat(v)
	case bool:
		return h.WriteBool(v)
	case error:
		return h.WriteError(v)
	case nil:
		return h.WriteNullString()
	case []any:
		return h.WriteArray(v)
	case map[string]any:
		return h.WriteMap(v)
	case big.Int:
		return h.WriteBigInt(v)
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
}

// WriteRaw writes raw RESP data to the connection.
// You can also use this function to check for any previous write errors by writing a nil or empty value.
func (h *ClientHandler) WriteRaw(v []byte) error {
	return h.write(v)
}
