package main

import (
	"bufio"
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment files
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatalln("Failed to read environment file!")
	}

	// Create a new server listening on the adress
	l, err := net.Listen("tcp4", "127.0.0.1:6969")
	if err != nil {
		log.Fatalln(err)
	}

	// Run hun that processes commands
	hub := Hub{
		req:    make(chan Request),
		users:  make(map[net.Conn]*User),
		verifs: make(map[net.Conn]string),
		db:     connectDB(),
	}
	go hub.Run()

	// Endless loop to listen for connections
	for {
		c, err := l.Accept()
		if err != nil {
			//* Error with the connection
			log.Println(err)
			continue // Keep seeking clients
		}

		cl := &gc.Connection{
			Conn: c,
			RD:   bufio.NewReader(c),
		}

		go ListenConnection(cl, hub.req)
	}
}
