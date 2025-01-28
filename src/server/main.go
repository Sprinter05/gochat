package main

import (
	"fmt"
	"log"
	"net"
)

type Client struct {
	conn net.Conn
	//addr net.IP
}

func main() {
	// Create a new server listening on the adress
	l, err := net.Listen("tcp", "127.0.0.1:6969")
	if err != nil {
		log.Fatal(err)
	}

	// Endless loop to listen for connections
	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			continue // Keep seeking clients
		}
		go handleClient(&Client{conn: c})
	}
}
