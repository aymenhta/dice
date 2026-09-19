package diceio

import (
	"bytes"
	"fmt"
	"io"
)

func SendError(w io.Writer, message string, args ...any) (int, error) {
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	var buff bytes.Buffer
	buff.WriteString("-")
	buff.WriteString(message)
	buff.WriteString("\r\n")
	return w.Write(buff.Bytes())
}

func RespondWithOk(w io.Writer) (int, error) {
	return w.Write([]byte("+OK\r\n"))
}

func RespondWithNullBulkStr(w io.Writer) (int, error) {
	return w.Write([]byte("$-1\r\n"))
}
