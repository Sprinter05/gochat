package main

import (
	"io"
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func processHeader(cl *gc.Connection, cmd *gc.Command) error {
	// Read header from the wire
	if err := cl.ListenHeader(cmd); err != nil {
		//* Error with header
		log.Println(err)
		// Connection closed by client
		if err == io.EOF {
			return err
		}
		// Send error packet to client
		sendErrorPacket(cmd.HD.Ord, err, cl.Conn)
	}
	return nil
}

func processPayload(cl *gc.Connection, cmd *gc.Command) error {
	// Read payload from the wire
	if err := cl.ListenPayload(cmd); err != nil {
		//* Error with payload
		log.Println(err)
		// Connection closed by client
		if err == io.EOF {
			return err
		}
		// Send error packet to client
		sendErrorPacket(cmd.HD.Ord, err, cl.Conn)
	}
	return nil
}

// Listens from a client and sends itself trough a channel for the hub to process
func ListenConnection(cl *gc.Connection, hub chan<- Request) {
	// Close connection when exiting
	defer cl.Conn.Close()

	for {
		cmd := gc.Command{}

		// Process the fields of the packet
		if processHeader(cl, &cmd) != nil {
			return
		}
		if processPayload(cl, &cmd) != nil {
			return
		}

		// Send OK reply to the client
		sendOKPacket(cmd.HD.Ord, cl.Conn)

		// Send command to the hub
		hub <- Request{
			cl:  cl.Conn,
			cmd: cmd,
		}
	}

}
