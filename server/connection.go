package main

import (
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/hubs"
)

/* COMMAND FUNCTIONS */

// Reads a header of a comamnd from the TCP connection
func withHeader(cl spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cmd.ListenHeader(cl); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("header", ip, err)

		// Send error packet
		pak, e := spec.NewPacket(spec.ERR, cmd.HD.ID, spec.ErrorCode(err))
		if e != nil {
			log.Packet(spec.ERR, e)
		} else {
			cl.Conn.Write(pak)
		}

		return err
	}
	return nil
}

// Reads the arguments of a command from the TCP connection
func withPayload(cl spec.Connection, cmd *spec.Command) error {
	// Reads using the reader assigned to the connection
	if err := cmd.ListenPayload(cl); err != nil {
		ip := cl.Conn.RemoteAddr().String()
		log.Read("payload", ip, err)

		// Send error packet
		pak, e := spec.NewPacket(spec.ERR, cmd.HD.ID, spec.ErrorCode(err))
		if e != nil {
			log.Packet(spec.ERR, e)
		} else {
			cl.Conn.Write(pak)
		}

		return err
	}
	return nil
}

// Reads from a connection and returns a command according to the specification,
// with all fields, or an error.
func wrapCommand(cl spec.Connection) (cmd spec.Command, err error) {
	ip := cl.Conn.RemoteAddr().String()

	// Error logged by the function
	read := new(spec.Command)
	if err = withHeader(cl, read); err != nil {
		return cmd, err
	}

	// Check that all header fields are correct
	if err := cmd.HD.ServerCheck(); err != nil {
		log.Read("header checking", ip, err)
		return cmd, err
	}

	// If there are no arguments we do not process the payload
	if cmd.HD.Args != 0 && cmd.HD.Len != 0 {
		// Error logged by the function
		if err = withPayload(cl, read); err != nil {
			return cmd, err
		}
	}

	return *read, nil
}

/* CONNECTION FUNCTIONS */

// Listens for packets from a client connection until the connection is shut down
func ListenConnection(cl spec.Connection, c *models.Counter, req chan<- hubs.Request, hub *hubs.Hub) {
	// Cleanup connection on exit
	defer func() {
		hub.Cleanup(cl.Conn)
		cl.Conn.Close()
		close(req)
		c.Dec()
		log.Connection(
			cl.Conn.RemoteAddr().String(),
			true,
		)
	}()

	// Set timeout and log connection
	ip := cl.Conn.RemoteAddr().String()
	deadline := time.Now().Add(time.Duration(spec.ReadTimeout) * time.Minute)
	log.Connection(
		cl.Conn.RemoteAddr().String(),
		false,
	)

	for {
		// Works as an idle timeout calling it each time
		err := cl.Conn.SetReadDeadline(deadline)
		if err != nil {
			log.Read("deadline setup", ip, err)
		}

		cmd, err := wrapCommand(cl)
		if err != nil {
			// Malformed, cleanup connection
			return
		}

		// Keep conection alive packet
		if cmd.HD.Op == spec.KEEP {
			continue
		}

		req <- hubs.Request{
			Conn:    cl.Conn,
			TLS:     cl.TLS,
			Command: cmd,
		}
	}

}

// Runs all commands for a single client
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

		hubs.Process(hub, r, *u)
	}
}
