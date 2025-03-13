package main

// Manages the client listener

import (
	"bufio"
	"io"
	"log"
	"net"

	"github.com/Sprinter05/gochat/gcspec"
)

// Buffer where packets whose response is pending are allocated
var PacketBuffer map[uint16]*[]byte = make(map[uint16]*[]byte)

// Starts listening for packets
func Listen(con net.Conn) {

	cl := &gcspec.Connection{
		Conn: con,
		RD:   bufio.NewReader(con),
	}

	defer cl.Conn.Close()

	for {
		pct := gcspec.Command{}

		// Reads header
		err := cl.ListenHeader(&pct)
		if err != nil {
			// Connection closed by server
			if err == io.EOF {
				return
			}
			continue // Continues listening
		}
		// Reads payload
		err = cl.ListenPayload(&pct)
		if err != nil {
			log.Print(err)
			// Connection closed by server
			if err == io.EOF {
				return
			}

			continue // Continues listening
		}
		// If the server packet was correct, by this point in the code, it has been completely received

		if IsVerbose {
			ClearPrompt()
			pct.ShellPrint()
			pct.Print()
		}

		// Clears shell prompt to print the packet
		ClearPrompt()
		// Prints received packet
		pct.ShellPrint()
		// Prints the shell prompt again
		PrintPrompt()
	}
}
