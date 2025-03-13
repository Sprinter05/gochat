package spec

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"time"
)

/* TYPES */

// Identifies a TCP established connection
// that can be communicated with using a buffered reader
// that is assigned to the connection when using
// the NewConnection() method.
//
// Note that a established connection does not imply a
// logged in user.
type Connection struct {
	Conn net.Conn
	RD   *bufio.Reader
	TLS  bool
}

// Specifies a message to be received
type Message struct {
	Sender  string
	Content []byte
	Stamp   time.Time
}

/* CONNECTION FUNCTIONS */

// Returns a new TCP connection with a buffered reader and
// TLS information using the connection from the [net]
// package as a base.
func NewConnection(cl net.Conn, tls bool) Connection {
	return Connection{
		Conn: cl,
		RD:   bufio.NewReader(cl),
		TLS:  tls,
	}
}

// Factory method that reads from a connection and modifies
// the header values of the command accordingly.
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

// Factory method that reads from a connection and modifies
// the arguments of the command accordingly.
func (cmd *Command) ListenPayload(cl Connection) error {
	var buf bytes.Buffer
	var tot int

	// Preallocate the arguments matrix
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
		buf.Grow(grow) // preallocation
		l, err := buf.Write(b)
		if err != nil || grow != l {
			// This implies the payload is too big
			// Or that the size does not match
			return err
		}

		// Single argument over limit
		if l > MaxArgSize {
			return ErrorMaxSize
		}

		// Check if the total payload size is too big
		tot += l
		if tot > MaxPayload {
			return ErrorMaxSize
		}

		// Check if it ends in CRLF
		// Also checks if it has at least 2 bytes
		if l >= 2 && b[l-2] == '\r' {
			total := buf.Bytes()
			size := buf.Len()

			// Preallocate new array
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
