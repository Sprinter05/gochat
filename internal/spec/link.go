package spec

import (
	"bufio"
	"bytes"
	"net"
	"os"
)

/* TYPES */

// Identifies a client in the server
type Connection struct {
	Conn net.Conn
	RD   *bufio.Reader
	TLS  bool
}

/* CONNECTION FUNCTIONS */

// Reads the header of a connection and verifies it is correct
func (cmd *Command) ListenHeader(cl Connection) error {
	// Read from the wire
	b, err := cl.RD.ReadBytes('\n')
	if err != nil {
		if err == os.ErrDeadlineExceeded {
			return ErrorIdle
		}
		return ErrorConnection
	}

	// Make sure the size is appropiate
	// We add 2 due to the delimiter
	if len(b) < HeaderSize+2 {
		return ErrorHeader
	}

	cmd.HD = NewHeader(b)

	// Header processed
	return nil
}

// Reads a payload to put it into a command
func (cmd *Command) ListenPayload(cl Connection) error {
	// Buffer and total length
	var buf bytes.Buffer
	var tot int

	// Allocate the arguments
	cmd.Args = make([][]byte, cmd.HD.Args)

	// Read until all arguments have been processed
	for i := 0; i < int(cmd.HD.Args); {
		// Read from the wire
		b, err := cl.RD.ReadBytes('\n')
		if err != nil {
			return ErrorConnection
		}

		// Write into the buffer and get length
		grow := len(b)
		buf.Grow(grow)
		l, err := buf.Write(b)
		if err != nil || grow != l {
			// This implies the payload is too big
			return err
		}

		// Single argument over limit
		if l > MaxArgSize {
			return ErrorMaxSize
		}

		// Check if the payload is too big
		tot += l
		if tot > MaxPayload {
			return ErrorMaxSize
		}

		// Check if it ends in CRLF
		// Also checks if it has at least 2 bytes
		if l >= 2 && b[l-2] == '\r' {
			total := buf.Bytes()
			size := buf.Len()

			// Allocate new array
			// Do not append CRLF
			cmd.Args[i] = make([]byte, size-2)
			copy(cmd.Args[i], total[:size-2])

			// Empty buffer and go to next argument
			buf.Reset()
			i++
		}
	}

	// Payload length incorrect
	if tot != int(cmd.HD.Len) {
		return ErrorArguments
	}

	// Payload processed
	return nil
}
