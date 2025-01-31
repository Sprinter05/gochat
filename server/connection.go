package main

import (
	"bufio"
	"log"
	"net"

	. "github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
	rd   *bufio.Reader
	req  chan Command
}

// Reads the header of a connection and verifies it is correct
func (cl *Client) listenHeader() {
	cmd := Command{}

	// Header processing
	for {
		// Read from the wire
		b, err := cl.rd.ReadBytes('\n')
		if err != nil {
			cl.conn.Close()
			log.Fatal(err)
			return
		}

		// Make sure the size is appropaite
		if len(b) < HeaderSize {
			pak, err := NewPacket(ERR, ErrorCode(ErrorHeader), nil)
			if err == nil {
				cl.conn.Write(pak)
			}
			continue
		}

		// Create and check the header
		cmd.HD = NewHeader(b)
		if err := cmd.HD.Check(); err != nil {
			log.Print(err)
			continue
		}

		break // Header processed
	}

	cl.listenPayload(&cmd)
}

// Reads a payload to put it into a command
func (cl *Client) listenPayload(cmd *Command) {
	defer cl.conn.Close()

	// Read until all arguments have been read
	for i := 0; i < int(cmd.HD.Args); {
		_, err := cl.rd.ReadBytes('\n')
		if err != nil {
			log.Print(err)
			continue
		}
	}
}
