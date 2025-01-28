package main

import (
	"log"
	"net"

	prot "github.com/Sprinter05/gochat/protocol"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
}

// Checks that the header contains valid data
func checkHeader(h prot.Header) error {
	// Check version
	if h.Version != prot.Version {
		return prot.ErrorVersion
	}

	// Check action code is valid
	if prot.GetClientActionCode(h.Action) == "" {
		return prot.ErrorInvalid
	}

	return nil
}

/*
Handles a connection with a client by verifying the
connection and then reading from it until closed
*/
func handleClient(client *Client) {
	defer client.conn.Close()

	// Check the header and size
	buffer := make([]byte, 4)
	_, err := client.conn.Read(buffer)
	if err != nil {
		log.Print(err)
		return
	}

	// Check the header
	hdr := prot.GetHeader(buffer)
	if err := checkHeader(hdr); err != nil {
		log.Print(err)
	}
}
