package main

import (
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func processHeader(cl *gc.Connection, cmd *gc.Command) error {
	// Read header from the wire
	if err := cl.ListenHeader(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Printf("Error reading header from %s: %s\n", ip, err)
		// Connection closed
		if err == gc.ErrorConnection {
			return err
		}
		// Incorrect header
		sendErrorPacket(cmd.HD.ID, gc.ErrorHeader, cl.Conn)
	}
	return nil
}

func processPayload(cl *gc.Connection, cmd *gc.Command) error {
	// Read payload from the wire
	if err := cl.ListenPayload(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Printf("Error reading paylaod from %s: %s\n", ip, err)
		// Connection closed
		if err == gc.ErrorConnection {
			return err
		}
		// Incorrect payload
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, cl.Conn)
	}
	return nil
}

// Listens from a client and sends itself trough a channel for the hub to process
func ListenConnection(cl *gc.Connection, hubreq chan<- Request, hubcl chan<- net.Conn) {
	// Close connection when exiting
	defer cl.Conn.Close()

	for {
		cmd := gc.Command{}

		// Process the fields of the packet
		if processHeader(cl, &cmd) != nil {
			// Cleanup connection
			hubcl <- cl.Conn
			return
		}
		if processPayload(cl, &cmd) != nil {
			// Cleanup connection
			hubcl <- cl.Conn
			return
		}

		// Check that it has enough arguments
		if int(cmd.HD.Args) != gc.IDToArgs(cmd.HD.Op) {
			sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, cl.Conn)
			return
		}

		// Send OK reply to the client
		sendOKPacket(cmd.HD.ID, cl.Conn)

		// Send command to the hub
		hubreq <- Request{
			cl:  cl.Conn,
			cmd: cmd,
		}
	}

}
