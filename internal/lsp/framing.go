package lsp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// maxMessage caps one message from a server (and from a socket).
const maxMessage = 16 << 20

// readMessage reads one Content-Length framed message from a language server.
func readMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line != "" {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if length < 0 {
				return nil, fmt.Errorf("message without Content-Length")
			}
			break
		}
		name, val, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil || n < 0 || n > maxMessage {
				return nil, fmt.Errorf("bad Content-Length %q", val)
			}
			length = n
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, fmt.Errorf("read message body: %w", err)
	}
	return body, nil
}

// writeMessage writes body with its Content-Length header in one write.
func writeMessage(w io.Writer, body []byte) error {
	var buf bytes.Buffer
	buf.Grow(len(body) + 32)
	buf.WriteString("Content-Length: ")
	buf.WriteString(strconv.Itoa(len(body)))
	buf.WriteString("\r\n\r\n")
	buf.Write(body)
	_, err := w.Write(buf.Bytes())
	return err
}
