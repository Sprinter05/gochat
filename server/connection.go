package main

import (
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
	req  chan gcspec.Command // Send only
}

// Handles a connection with a client by verifying the
// connection and then reading from it until closed
func (cl *Client) readHeader() {
	defer cl.conn.Close()
}
