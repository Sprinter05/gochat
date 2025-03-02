package gcspec

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
}

/* CONNECTION FUNCTIONS */

// Reads the header of a connection and verifies it is correct
func (cl *Connection) ListenHeader(cmd *Command) error {
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

	// Create and check the header
	cmd.HD = NewHeader(b)
	if err := cmd.HD.Check(); err != nil {
		return err
	}

	// Header processed
	return nil
}

// Reads a payload to put it into a command
func (cl *Connection) ListenPayload(cmd *Command) error {
	// Buffer and total length
	var buf bytes.Buffer
	var tot int

	// Allocate the arguments
	cmd.Args = make([]Arg, cmd.HD.Args)

	// Read until all arguments have been processed
	for i := 0; i < int(cmd.HD.Args); {
		// Read from the wire
		b, err := cl.RD.ReadBytes('\n')
		if err != nil {
			return ErrorConnection
		}

		// Write into the buffer and get length
		l, err := buf.Write(b)
		if err != nil {
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
		if len(b) >= 2 && string(b[l-2]) == "\r" {
			b := buf.Bytes()
			siz := buf.Len()

			// Allocate new array
			mem := make([]byte, siz)
			copy(mem, b)

			// Do not append CRLF
			cmd.Args[i] = mem[:siz-2]
			buf.Reset() // Empty the buffer
			i++         // Next argument
		}
	}

	// Payload length incorrect
	if tot != int(cmd.HD.Len) {
		return ErrorArguments
	}

	// Payload processed
	return nil
}
