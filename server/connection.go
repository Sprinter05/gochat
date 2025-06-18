package main

import (
	"crypto/tls"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/hubs"
)

/* INITIAL CONNECTION */

// Waits for a possible TLS handshake and sends an initial welcome OK
func welcomeConn(cl *spec.Connection) {
	// Set timeout for the initial write to prevent blocking forever
	deadline := time.Now().Add(time.Duration(spec.HandshakeTimeout) * time.Second)
	cl.Conn.SetDeadline(deadline)

	// Notify the user they are connected to the server
	pak, err := spec.NewPacket(spec.OK, spec.NullID, spec.EmptyInfo)
	if err != nil {
		log.Packet(spec.OK, err)
	} else {
		_, err := cl.Conn.Write(pak)
		if err != nil {
			log.Error("handshake with new connection", err)
		}
	}

	// Disable timeout as it is only for the first write
	cl.Conn.SetDeadline(time.Time{})

	// Check if its a TLS connection
	_, ok := cl.Conn.(*tls.Conn)
	cl.TLS = ok
}

/* COMMAND FUNCTIONS */

// Reads from a connection and returns a command according to the
// specification, with all fields, or an error.
func readCommand(cl spec.Connection) (cmd spec.Command, err error) {
	ip := cl.Conn.RemoteAddr().String()

	// Error logged by the function
	if err := cmd.ListenHeader(cl); err != nil {
		log.Read("header", ip, err)
		hubs.SendErrorPacket(cmd.HD.ID, err, cl.Conn)
		return cmd, err
	}

	// Check that all header fields are correct
	if err := cmd.HD.ServerCheck(); err != nil {
		log.Read("header checking", ip, err)
		hubs.SendErrorPacket(cmd.HD.ID, err, cl.Conn)
		return cmd, err
	}

	// If there are no arguments we do not process the payload
	if cmd.HD.Args != 0 && cmd.HD.Len != 0 {
		// Error logged by the function
		if err := cmd.ListenPayload(cl); err != nil {
			log.Read("payload", ip, err)
			hubs.SendErrorPacket(cmd.HD.ID, err, cl.Conn)
			return cmd, err
		}
	}

	return cmd, nil
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

	// Perform initial welcome handshake
	welcomeConn(&cl)

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

		cmd, err := readCommand(cl)
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
			hubs.SendErrorPacket(r.Command.HD.ID, err, r.Conn)
			log.Error("session checking for "+ip, err)
			continue // Next request
		}

		hubs.Process(hub, r, *u)
	}
}
