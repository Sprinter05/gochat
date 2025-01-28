package main

import (
	"log"

	prot "github.com/Sprinter05/gochat/protocol"
)

func checkHeader(h []byte) error {
	hdr := GetHeader(h[:prot.HeaderSize])

	// Check version
	if hdr.version != prot.Version {
		return prot.ErrorVersion
	}

	// Check action code is valid
	if prot.GetClientActionCode(hdr.action) == "" {
		return prot.ErrorInvalid
	}

	return nil
}

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
	if err := checkHeader(buffer); err != nil {
		log.Print(err)
	}
}
