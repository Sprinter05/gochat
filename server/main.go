package main

import (
	"bufio"
	"fmt"
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

func main() {
	// Create a new server listening on the adress
	l, err := net.Listen("tcp4", "127.0.0.1:6969")
	if err != nil {
		log.Fatal(err)
	}

	// Run hun that processes commands
	hub := Hub{
		comm: make(chan Request),
	}
	go hub.Run()

	// Endless loop to listen for connections
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue // Keep seeking clients
		}

		cl := &gc.Connection{
			Conn: c,
			RD:   bufio.NewReader(c),
		}

		go listenConnection(cl, hub.comm)
	}
}
