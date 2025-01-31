package main

import (
	"fmt"
	"log"
	"net"
)

func main() {
	// Create a new server listening on the adress
	l, err := net.Listen("tcp", "127.0.0.1:6969")
	if err != nil {
		log.Fatal(err)
	}

	// Run hun that processes commands
	hub := Hub{
		comm: make(chan Client),
	}
	hub.Run()

	// Endless loop to listen for connections
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue // Keep seeking clients
		}

		cl := &Client{
			conn: c,
		}

		go cl.Listen(hub.comm)
	}
}
