package main

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/model"
)

// FUNCTIONS

func processHeader(cl *spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cl.ListenHeader(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("header", ip, err)
		sendErrorPacket(cmd.HD.ID, err, cl.Conn)
		return err
	}
	return nil
}

func processPayload(cl *spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cl.ListenPayload(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("payload", ip, err)
		sendErrorPacket(cmd.HD.ID, err, cl.Conn)
		return err
	}
	return nil
}

// Cleans up the connection upon exit
func cleanup(cl net.Conn, c *model.Counter, ch chan<- Request, hub chan<- net.Conn) {
	// Close the requests channel
	close(ch)

	// Request cleaning the tables
	hub <- cl

	// Close connection
	cl.Close()

	// Decrease amount of connected clients
	c.Dec()

	// Log connection close
	log.Connection(
		cl.RemoteAddr().String(),
		true,
	)
}

// Listens from a client and communicates with the hub through the channels
func ListenConnection(cl *spec.Connection, c *model.Counter, req chan<- Request, hubcl chan<- net.Conn) {
	// Cleanup connection on error
	defer cleanup(cl.Conn, c, req, hubcl)

	// Check if the TLS is valid
	_, ok := cl.Conn.(*tls.Conn)
	if !ok {
		log.IP("failed tls verification", cl.Conn.RemoteAddr())
	}

	// Timeout
	deadline := time.Now().Add(time.Duration(spec.ReadTimeout) * time.Minute)

	// Log connection
	log.Connection(
		cl.Conn.RemoteAddr().String(),
		false,
	)

	for {
		ip := cl.Conn.RemoteAddr().String()
		cmd := new(spec.Command)

		// Works as an idle timeout calling it each time
		cl.Conn.SetReadDeadline(deadline)

		if processHeader(cl, cmd) != nil {
			return
		}

		// Check that all header fields are correct
		if err := cmd.HD.ServerCheck(); err != nil {
			log.Read("header checking", ip, err)
		}

		// If there are no arguments we do not process the payload
		if cmd.HD.Args != 0 && cmd.HD.Len != 0 {
			if processPayload(cl, cmd) != nil {
				return
			}
		}

		// Keep conection alive packet
		if cmd.HD.Op == spec.KEEP {
			continue
		}

		req <- Request{
			cl:  cl.Conn,
			tls: cl.TLS,
			cmd: *cmd,
		}
	}

}

// Catches up messages for the logged connection
func catchUp(cl net.Conn, id spec.ID, msgs ...model.Message) error {
	for _, v := range msgs {
		// Turn timestamp to byte array and create packet
		stp := spec.UnixStampToBytes(v.Stamp)

		pak, err := spec.NewPacket(spec.RECIV, id, spec.EmptyInfo,
			spec.Arg(v.Sender),
			spec.Arg(stp),
			spec.Arg(v.Content),
		)
		if err != nil {
			log.Packet(spec.RECIV, err)
			return err
		}
		cl.Write(pak)
	}

	return nil
}

// Wraps concurrency with each client's command
func runTask(hub *Hub, req <-chan Request) {
	for r := range req {
		// Show request
		ip := r.cl.RemoteAddr().String()
		log.Request(ip, r.cmd)

		// Check if the user can be served
		u, err := hub.CheckSession(r)
		if err != nil {
			log.Error("session checking for "+ip, err)
			continue // Next request
		}

		procRequest(hub, r, u)
	}
}
