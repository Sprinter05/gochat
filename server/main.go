package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"

	gc "github.com/Sprinter05/gochat/gcspec"
	"github.com/joho/godotenv"
)

// Global log
var gclog Logging

// Sets up logging
// Reads environment file from first cli argument
// init() always runs when the program starts
func init() {
	// If we default to stderr it won't print unless debugged
	log.SetOutput(os.Stdout)

	if len(os.Args) < 2 {
		// No environment file supplied
		gclog.Fatal("loading env file", ErrorCLIArgs)
	}

	// Argument 0 is the pathname to the executable
	err := godotenv.Load(os.Args[1])
	if err != nil {
		gclog.Fatal("env file reading", err)
	}

	// Setup logging levels
	// No need to check if the env var exists
	// We just default to FATAL
	lv := os.Getenv("LOG_LEVL")
	switch lv {
	case "ALL":
		gclog = ALL
	case "INFO":
		gclog = INFO
	case "ERROR":
		gclog = ERROR
	default:
		gclog = FATAL
		lv = "FATAL"
	}
	fmt.Printf("-> Logging with log level %s...\n", lv)
}

// Creates a hub with all channels, caches and database
// Indicates the hub to start running
func setupHub() *Hub {
	// Allocate all data structures
	hub := Hub{
		clean:  make(chan net.Conn, gc.MaxClients/2),
		shtdwn: make(chan bool),
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

// Creates a listener for the socket
func setupConn() net.Listener {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		gclog.Environ("SRV_ADDR")
	}

	port, ok := os.LookupEnv("SRV_PORT")
	if !ok {
		gclog.Environ("SRV_PORT")
	}

	socket := fmt.Sprintf(
		"%s:%s",
		addr,
		port,
	)

	l, err := net.Listen("tcp4", socket)
	if err != nil {
		gclog.Fatal("socket setup", err)
	}

	return l
}

// Create a TLS listener for the socket
func setupTLSConn() net.Listener {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		gclog.Environ("SRV_ADDR")
	}

	port, ok := os.LookupEnv("TLS_PORT")
	if !ok {
		gclog.Environ("TLS_PORT")
	}

	socket := fmt.Sprintf(
		"%s:%s",
		addr,
		port,
	)

	cert, err := tls.LoadX509KeyPair(
		os.Getenv("TLS_CERT"),
		os.Getenv("TLS_KEYF"),
	)
	if err != nil {
		gclog.Fatal("tls loading", err)
	}
	config := tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	l, err := tls.Listen("tcp4", socket, &config)
	if err != nil {
		gclog.Fatal("tls socket setup", err)
	}

	return l
}

func run(l net.Listener, hub *Hub, count *Counter) {
	for {
		// If we exceed the client count we just wait until a spot is free
		if count.Get() == gc.MaxClients {
			continue
		}

		c, err := l.Accept()
		if err != nil {
			gclog.Error("connection accept", err)
			// Keep accepting clients
			continue
		}
		count.Inc()

		// Check if its tls
		_, ok := c.(*tls.Conn)

		cl := &gc.Connection{
			Conn: c,
			RD:   bufio.NewReader(c),
			TLS:  ok,
		}

		// Buffered channel for intercommunication
		req := make(chan Request, MaxUserRequests)

		// Listens to the client's packets
		go ListenConnection(cl, count, req, hub.clean)

		// Runs the client's commands
		go runTask(hub, req)
	}
}

// TODO: Reusable verif tokens (TLS only)
// TODO: Both TLS and non-TLS ports

func main() {
	sock := setupConn()
	tlssock := setupTLSConn()

	// Create hub
	hub := setupHub()

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for connections! --\n")

	// Endless loop to listen for connections
	count := new(Counter)
	run(sock, hub, count)
	run(tlssock, hub, count)
}
