package main

import (
	"log"
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Identifies a client in the server
type Client struct {
	conn net.Conn
}

// Checks that the header contains valid data
func checkHeader(h gcspec.Header) error {
	if h.Version != gcspec.SpecVersion {
		return gcspec.ErrorVersion
	}

	if gcspec.ClientActionCode(h.Action) == "" {
		return gcspec.ErrorInvalid
	}

	return nil
}

// Handles a connection with a client by verifying the
// connection and then reading from it until closed
func handleClient(client *Client) {
	defer client.conn.Close()

	// Check the header and size according to the spec
	siz := gcspec.HeaderSize + gcspec.LengthSize
	buf := make([]byte, 0, siz)
	for {
		n, err := client.conn.Read(buf)
		if err != nil {
			log.Print(err)
			return
		}

		// Repeat until its properly sized
		if n < siz {
			continue
		}
		break
	}

	// Return an error packet when necessary
	hdr := gcspec.NewHeader(buf)
	if err := checkHeader(hdr); err != nil {
		log.Print(err)
	}
}
