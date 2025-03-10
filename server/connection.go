package main

import (
	"crypto/tls"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/hubs"
	"github.com/Sprinter05/gochat/server/model"
)

// FUNCTIONS

func processHeader(cl *spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cl.ListenHeader(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("header", ip, err)

		// Send error packet
		pak, e := spec.NewPacket(spec.ERR, cmd.HD.ID, spec.ErrorCode(err), nil)
		if e != nil {
			log.Packet(spec.ERR, e)
		} else {
			cl.Conn.Write(pak)
		}

		return err
	}
	return nil
}

func processPayload(cl *spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cl.ListenPayload(cmd); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("payload", ip, err)

		// Send error packet
		pak, e := spec.NewPacket(spec.ERR, cmd.HD.ID, spec.ErrorCode(err), nil)
		if e != nil {
			log.Packet(spec.ERR, e)
		} else {
			cl.Conn.Write(pak)
		}

		return err
	}
	return nil
}

// Listens from a client and communicates with the hub through the channels
func ListenConnection(cl *spec.Connection, c *model.Counter, req chan<- hubs.Request, hub *hubs.Hub) {
	// Cleanup connection on exit
	defer func() {
		// Close the requests channel
		close(req)
		// Request cleaning the tables
		hub.Cleanup(cl.Conn)
		// Close connection socket
		cl.Conn.Close()
		// Decrease amount of connected clients
		c.Dec()
		// Log connection close
		log.Connection(
			cl.Conn.RemoteAddr().String(),
			true,
		)
	}()

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

		req <- hubs.Request{
			Conn:    cl.Conn,
			TLS:     cl.TLS,
			Command: *cmd,
		}
	}

}

// Wraps concurrency with each client's command
func RunTask(hub *hubs.Hub, req <-chan hubs.Request) {
	for r := range req {
		// Show request
		ip := r.Conn.RemoteAddr().String()
		log.Request(ip, r.Command)

		// Check if the user can be served
		u, err := hub.Session(r)
		if err != nil {
			log.Error("session checking for "+ip, err)
			continue // Next request
		}

		hubs.Process(hub, r, u)
	}
}
