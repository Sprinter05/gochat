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
// init() always runs when the program starts
func init() {
	// If we default to stderr it won't print unless debugged
	log.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		log.Fatalf("Not enough arguments supplied!")
		os.Exit(1)
	}

	// Argument 0 is the pathname to the executable
	err := godotenv.Load(os.Args[1])
	if err != nil {
		log.Fatalf("Failed to read environment file: %s\n", err)
		os.Exit(1)
	}
}

// Creates a hub with all channels, caches and database
// Indicates the hub to start running
func setupHub() *Hub {
	// Allocate all data structures
	hub := Hub{
		req:   make(chan Request),
		clean: make(chan net.Conn),
		users: table[*User]{
			tab: make(map[net.Conn]*User),
		},
		verifs: table[*Verif]{
			tab: make(map[net.Conn]*Verif),
		},
		runners: table[chan Task]{
			tab: make(map[net.Conn]chan Task),
		},
		db: connectDB(),
	}

	go hub.Start()

	return &hub
}

func main() {
	addr := fmt.Sprintf(
		"%s:%s",
		os.Getenv("SRV_ADDR"),
		os.Getenv("SRV_PORT"),
	)
	l, err := net.Listen("tcp4", addr)
	if err != nil {
		log.Fatalln(err)
	}

	hub := setupHub()

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for incoming connections! --\n")

	// Endless loop to listen for connections
	var count int
	for {
		// If we exceed the client count we just wait until a spot is free
		if count == gc.MaxClients {
			continue
		}

		c, err := l.Accept()
		if err != nil {
			log.Println(err)
			// Keep accepting clients
			continue
		}
		count++

		cl := &gc.Connection{
			Conn: c,
			RD:   bufio.NewReader(c),
		}

		go ListenConnection(cl, hub.req, hub.clean)

		// Create runner that processes commands
		send := make(chan Task)
		hub.runners.Add(cl.Conn, send)
		go runTask(send)
	}
}
