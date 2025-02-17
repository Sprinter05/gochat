package main

import (
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func processHeader(cl *gc.Connection, cmd *gc.Command) error {
	// Reads using the reader assigned to the connection
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
	// Reads using the reader assigned to the connection
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

// Listens from a client and communicates with the hub through the channels
func ListenConnection(cl *gc.Connection, hubreq chan<- Request, hubcl chan<- net.Conn) {
	defer cl.Conn.Close()

	for {
		cmd := new(gc.Command)

		if processHeader(cl, cmd) != nil {
			// Cleanup connection on error
			hubcl <- cl.Conn
			return
		}
		if processPayload(cl, cmd) != nil {
			// Cleanup connection on error
			hubcl <- cl.Conn
			return
		}

		// Check that it has enough arguments unless its the admin command
		if cmd.HD.Op != gc.ADMIN && (int(cmd.HD.Args) != gc.IDToArgs(cmd.HD.Op)) {
			sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, cl.Conn)
			return
		}

		hubreq <- Request{
			cl:  cl.Conn,
			cmd: *cmd,
		}
	}

}

// Catches up messages for the logged connection
func catchUp(cl net.Conn, msgs *[]Message, id gc.ID) error {
	for _, v := range *msgs {
		// Turn timestamp to byte array and create packet
		stp := gc.UnixStampToBytes(v.stamp)
		arg := []gc.Arg{
			gc.Arg(v.sender),
			gc.Arg(stp),
			gc.Arg(v.message),
		}

		// The packet ID is not used for RECIV
		pak, err := gc.NewPacket(gc.RECIV, id, gc.EmptyInfo, arg)
		if err != nil {
			log.Printf("Error when creating RECIV packet: %s\n", err)
			return err
		}
		cl.Write(pak)
	}

	return nil
}

// Wraps concurrency with each client's command
func runTask(ch <-chan Task) {
	// Will stop if the channel is closed
	for t := range ch {
		t.fun(t.hub, t.user, t.cmd)
	}
}
