package utils

import (
	"fmt"
	"math"
	"strconv"
	"time"
	"unsafe"

	"aymenhta.com/dice/internal/resp/static"
	"github.com/zeebo/xxh3"
)

// SanitizeSimpleString replaces all '\r' and '\n' characters with spaces.
// Useful when writing untrusted error messages to the client.
func SanitizeSimpleString(buf []byte) []byte {
	for i := 0; i < len(buf); i++ {
		if buf[i] == '\r' || buf[i] == '\n' {
			buf[i] = ' '
		}
	}
	return buf
}

// BStr converts a byte slice to a string without copying.
func BStr(bs []byte) string {
	p := unsafe.SliceData(bs)
	return unsafe.String(p, len(bs))
}

func BStrLowered(bs []byte) string {
	p := unsafe.SliceData(BytesToLower(bs))
	return unsafe.String(p, len(bs))
}

// StrB converts a string to a byte slice without copying.
func StrB(s string) []byte {
	if s == "" {
		return static.NullBytes
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func BytesToLower(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] + 32
		}
	}

	return b
}

func CopyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// Expand works similarly to slices.Grow but returns the expanded slice rather than the capped slice.
func Expand[S ~[]E, E any](s S, n int) S {
	if n < 0 {
		panic("cannot be negative")
	}
	if n -= len(s); n > 0 {
		s = append(s, make([]E, n)...)
	}
	return s
}

func ParseInt64(b []byte) (int64, error) {
	l := len(b)
	if l == 0 || l > 20 {
		return 0, strconv.ErrSyntax
	}
	v := uint64(0)
	c := b[0] - '0'
	neg := false
	switch {
	case c == 253: // '-' - '0'
		if l == 1 {
			return 0, strconv.ErrSyntax
		}
		neg = true
	case c > 9: // Not a digit
		return 0, strconv.ErrSyntax
	case c == 0: // '0'
		if l == 1 {
			return 0, nil
		}
		return 0, strconv.ErrSyntax
	default:
		v = uint64(c)
	}

	for i := 1; i < l; i++ {
		if i >= 19 && v > (math.MaxUint64/10) {
			// Overflow
			return 0, strconv.ErrRange
		}
		c = b[i] - '0'
		if c > 9 {
			// Not a digit
			return 0, strconv.ErrSyntax
		}
		ic := uint64(c)
		v *= 10
		if i >= 19 && v > math.MaxUint64-ic {
			// Overflow
			return 0, strconv.ErrRange
		}
		v += ic
	}

	if neg {
		if v > uint64((-(math.MinInt64 + 1))+1) {
			// Overflow
			return 0, strconv.ErrRange
		}
		return -int64(v), nil
	}
	if v > math.MaxInt64 {
		// Overflow
		return 0, strconv.ErrRange
	}
	return int64(v), nil
}

func ParseUInt64(b []byte) (uint64, error) {
	l := len(b)
	if l == 0 || l > 20 {
		return 0, strconv.ErrSyntax
	}

	// Handle zero and reject leading zeros.
	c := b[0] - '0'
	if c > 9 {
		return 0, strconv.ErrSyntax
	}
	if c == 0 {
		if l == 1 {
			return 0, nil
		}
		return 0, strconv.ErrSyntax
	}

	v := uint64(c)

	for i := 1; i < l; i++ {
		c = b[i] - '0'
		if c > 9 {
			return 0, strconv.ErrSyntax
		}

		digit := uint64(c)

		// Check whether v*10 + digit would overflow uint64.
		if v > math.MaxUint64/10 ||
			(v == math.MaxUint64/10 && digit > math.MaxUint64%10) {
			return 0, strconv.ErrRange
		}

		v = v*10 + digit
	}

	return v, nil
}

func ParseUInt32(b []byte) (uint32, error) {
	l := len(b)
	if l == 0 || l > 10 {
		return 0, strconv.ErrSyntax
	}

	n := uint64(0)
	for _, c := range b {
		c -= '0'
		if c > 9 {
			return 0, strconv.ErrSyntax
		}
		n = n*10 + uint64(c)
	}

	if n > math.MaxUint32 {
		return 0, strconv.ErrRange
	}

	return uint32(n), nil
}

func ParseInt32(b []byte) (int32, error) {
	l := len(b)
	if l == 0 || l > 11 {
		return 0, strconv.ErrSyntax
	}
	v := uint32(0)
	c := b[0] - '0'
	neg := false
	switch {
	case c == 253: // '-' - '0'
		if l == 1 {
			return 0, strconv.ErrSyntax
		}
		neg = true
	case c > 9: // Not a digit
		return 0, strconv.ErrSyntax
	case c == 0: // '0'
		if l == 1 {
			return 0, nil
		}
		return 0, strconv.ErrSyntax
	default:
		v = uint32(c)
	}

	for i := 1; i < l; i++ {
		if i >= 10 && v > (math.MaxUint32/10) {
			// Overflow
			return 0, strconv.ErrRange
		}
		c = b[i] - '0'
		if c > 9 {
			// Not a digit
			return 0, strconv.ErrSyntax
		}
		ic := uint32(c)
		v *= 10
		if i >= 10 && v > math.MaxUint32-ic {
			// Overflow
			return 0, strconv.ErrRange
		}
		v += ic
	}
	if neg {
		if v > uint32((-(math.MinInt32 + 1))+1) {
			// Overflow
			return 0, strconv.ErrRange
		}
		return -int32(v), nil
	}
	if v > math.MaxInt32 {
		// Overflow
		return 0, strconv.ErrRange
	}
	return int32(v), nil
}

func Pointer[T any](v T) *T {
	return &v
}

func WrongArgsNum(cmd string) error {
	return fmt.Errorf("wrong number of arguments for '%s' command", cmd)
}

func InvalidExpire(cmd string) error {
	return fmt.Errorf("invalid expire time in '%s' command", cmd)
}

// unixMsToInstant converts an absolute unix timestamp (ms) into a monotonic deadline.
// A timestamp already in the past yields "now", i.e., already expired.
func UnixMsToInstant(targetMs int64) time.Time {
	now := time.Now()
	nowMs := now.UnixNano() / int64(time.Millisecond)

	delta := targetMs - nowMs
	if delta <= 0 {
		return now
	}

	return now.Add(time.Duration(delta) * time.Millisecond)
}

// Add64 checks for overflow before returning the sum
func Add64(a, b int64) (int64, bool) {
	sum := a + b

	// If 'a' and 'b' have the same sign, but 'sum' has a different sign,
	// a signed integer overflow/underflow occurred.
	if (a^sum)&(b^sum) < 0 {
		return 0, false // Overflow occurred
	}

	return sum, true // Safe addition
}

func Int64ToBytes(val int64) []byte {
	return strconv.AppendInt(nil, val, 10)
}

func BytesToXXH3hex(b []byte) string {
	hashBytes := xxh3.Hash(b)
	return fmt.Sprintf("%016x", hashBytes)
}

func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}
