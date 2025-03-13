package main

import (
	"log"
	"net"
)

// Main method for the client

func main() {
	// Connects to the server
	con, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		log.Fatal(err)
	}
	// Closes conection once execution is over
	defer con.Close()

	// Starts listening for server packets
	go Listen(con, ctx, pctReceived)
	// Opens a shell
	NewShell(con, ctx, pctReceived)
}
