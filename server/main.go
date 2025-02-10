package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"

	gc "github.com/Sprinter05/gochat/gcspec"
	"github.com/joho/godotenv"
)

// Sets up logging
// Reads environment file from first cli argument
func setupEnv() {
	// Set up logging
	log.SetOutput(os.Stdout)

	// Load environment files
	err := godotenv.Load(os.Args[1])
	if err != nil {
		log.Fatalln("Failed to read environment file!")
	}
}

// Creates a hub with all channels, caches and database
// Indicates the hub to start running
func setupHub() *Hub {
	// Run hun that processes commands
	hub := Hub{
		req:   make(chan Request),
		clean: make(chan net.Conn),
		users: table[*User]{
			tab: make(map[net.Conn]*User),
		},
		verifs: table[*Verif]{
			tab: make(map[net.Conn]*Verif),
		},
		db: connectDB(),
	}
	go hub.Start()

	return &hub
}

func main() {
	// Create a new server listening on the adress
	l, err := net.Listen("tcp4", "127.0.0.1:6969")
	if err != nil {
		log.Fatalln(err)
	}

	// Setup the server
	setupEnv()
	hub := setupHub()

	// Print that the server is up and running
	fmt.Printf("-- Server running and listening for incoming connections! --\n")

	// Endless loop to listen for connections
	for {
		c, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue // Keep accepting clients
		}

		// Set up new connection
		cl := &gc.Connection{
			Conn: c,
			RD:   bufio.NewReader(c),
		}

		// Concurrnetly listen to that client
		go ListenConnection(cl, hub.req, hub.clean)
	}
}
