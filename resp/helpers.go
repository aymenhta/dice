package resp

func isDigit(b byte) bool {
	return '0' <= b && b <= '9'
}

func safeIndex(input []byte, pos int) byte {
	if pos < len(input) {
		return input[pos]
	}
	return '\x00'
}
