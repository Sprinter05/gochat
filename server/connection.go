package main

import (
	"bufio"
	"fmt"
	"log"
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
	req  chan gcspec.Command
}

// Handles a connection with a client by verifying the
// connection and then reading from it until closed
func (cl *Client) readHeader() {
	defer cl.conn.Close()

	for {
		b, err := bufio.NewReader(cl.conn).ReadBytes('\n')
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%s", b)
	}
}
