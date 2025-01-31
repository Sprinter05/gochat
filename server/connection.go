package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"strings"

	. "github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
	rd   *bufio.Reader
	cmd  Command
}

// Listens from a client and sends itself trough a channel for the hub to process
func (cl *Client) Listen(hub chan<- Client) {
	// Close connection when exiting
	defer cl.conn.Close()

	// Create channel and reader
	cl.rd = bufio.NewReader(cl.conn)

	for {
		cmd := Command{}

		// Read header from the wire
		if err := cl.listenHeader(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := NewPacket(ERR, ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.conn.Write(pak)
			}
			continue
		}

		// Read payload from the wire
		if err := cl.listenPayload(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := NewPacket(ERR, ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.conn.Write(pak)
			}
			continue
		}

		// Send command to the hub
		cl.cmd = cmd
		hub <- *cl
	}

}

// Reads the header of a connection and verifies it is correct
func (cl *Client) listenHeader(cmd *Command) error {
	// Read from the wire
	b, err := cl.rd.ReadBytes('\n')
	if err != nil {
		return err
	}

	// Make sure the size is appropaite
	if len(b) < HeaderSize {
		return ErrorHeader
	}

	// Create and check the header
	cmd.HD = NewHeader(b)
	if err := cmd.HD.Check(); err != nil {
		return ErrorHeader
	}

	// Header processed
	return nil
}

// Reads a payload to put it into a command
func (cl *Client) listenPayload(cmd *Command) error {
	// Buffer and total length
	var buf bytes.Buffer
	var tot int

	// Allocate the arguments
	cmd.Args = make([]string, cmd.HD.Args)

	// Read until all arguments have been processed
	for i := 0; i < int(cmd.HD.Args); {
		//? Check if the reader keeps the previous contents
		b, err := cl.rd.ReadBytes('\n')
		if err != nil {
			return err
		}

		// Write into the buffer and get length
		l, err := buf.Write(b)
		if err != nil {
			//! May cause unexpected behaviour
			buf.Reset()
			continue
		}
		tot += l

		// Check if the payload is too big
		if tot > MaxPayload {
			return ErrorMaxSize
		}

		// Check if it ends in CRLF
		if string(b[l-2]) == "\r" {
			// Append all necessary contents
			cmd.Args[i] = strings.Clone(buf.String())
			buf.Reset() // Empty the buffer
			i++         // Next argument
		}
	}

	// Payload processed
	return nil
}
