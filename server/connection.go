package main

import (
	"bufio"
	"log"
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
	rd   *bufio.Reader
	req  chan gcspec.Command
}

// Handles a connection with a client by verifying the
// connection and then reading from it until closed
func (cl *Client) listen() {
	defer cl.conn.Close()
	cmd := gcspec.Command{}

	// Header processing
	for {
		b, err := cl.rd.ReadBytes('\n')
		if err != nil {
			log.Fatal(err)
		}

		// Make sure the size is appropaite
		if len(b) < gcspec.HeaderSize {
			//TODO: Send error packet
			continue
		}

		cmd.HD = gcspec.NewHeader(b)
		if err := cmd.HD.Check(); err != nil {
			log.Print(err)
			continue
		}

	}
}
