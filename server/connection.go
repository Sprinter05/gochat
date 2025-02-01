package main

import (
	"io"
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

type Request struct {
	cl  *gc.Connection
	cmd gc.Command
}

// Listens from a client and sends itself trough a channel for the hub to process
func listenConnection(cl *gc.Connection, hub chan<- Request) {
	// Close connection when exiting
	defer cl.Conn.Close()

	for {
		cmd := gc.Command{}

		// Read header from the wire
		if err := cl.ListenHeader(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := gc.NewPacket(gc.ERR, gc.ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.Conn.Write(pak)
			}
			continue
		}

		// Read payload from the wire
		if err := cl.ListenPayload(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := gc.NewPacket(gc.ERR, gc.ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.Conn.Write(pak)
			}
			continue
		}

		// TODO: Send OK response to client
		// Send command to the hub
		hub <- Request{
			cl:  cl,
			cmd: cmd,
		}
	}

}
