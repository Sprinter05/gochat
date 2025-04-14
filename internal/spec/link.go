package spec

import (
	"bytes"
	"io"
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
	Conn net.Conn // TCP connection
	TLS  bool     // Whether it is connected through TLS
}

// Specifies a message that can be sent between clients
// and that is either sent directly through the server
// or stored in the database.
type Message struct {
	Sender  string    // Person that sent the message
	Content []byte    // Encrypted content
	Stamp   time.Time // Specifies when the message was sent
}

/* CONNECTION FUNCTIONS */

// Returns a new TCP connection with a buffered reader and
// TLS information using the connection from the [net]
// package as a base.
func NewConnection(cl net.Conn, tls bool) Connection {
	return Connection{
		Conn: cl,
		TLS:  tls,
	}
}

// Factory method that reads from a connection and modifies
// the header values of the command accordingly.
func (cmd *Command) ListenHeader(cl Connection) error {
	// Read from the wire accounting for CRLF
	b := make([]byte, HeaderSize+2)
	_, err := io.ReadAtLeast(cl.Conn, b, HeaderSize+2)
	if err != nil {
		if err == os.ErrDeadlineExceeded {
			return ErrorIdle
		}
		return ErrorConnection
	}

	// Make sure the size is appropiate
	// We add 2 due to CRLF
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
	// Read from the wire "Len" bytes
	b := make([]byte, cmd.HD.Len)
	_, err := io.ReadAtLeast(cl.Conn, b, int(cmd.HD.Len))
	if err != nil {
		if err == os.ErrDeadlineExceeded {
			return ErrorIdle
		}
		return ErrorConnection
	}

	// Split generates an extra empty argument so we get rid of it
	cmd.Args = (bytes.Split(b, []byte("\r\n")))[:cmd.HD.Args]
	if err := cmd.CheckArgs(); err != nil {
		return err
	}

	// Payload processed
	return nil
}
