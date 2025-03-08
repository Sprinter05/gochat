package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// Global log
var gclog Logging

// Sets up logging
// Reads environment file from first cli argument
// init() always runs when the program starts
func init() {
	// If we default to stderr it won't print unless debugged
	log.SetOutput(os.Stdout)

	// Argument 0 is the pathname to the executable
	if len(os.Args) > 1 {
		// Read argument 1 as .env if it exists
		err := godotenv.Load(os.Args[1])
		if err != nil {
			gclog.Fatal("env file reading", err)
		}
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
	fmt.Printf("-> Logging with log level %s\n", lv)
}

// Creates a log file and returns both file and log
func logFile() *os.File {
	f, ok := os.LookupEnv("DB_LOGF")
	if !ok {
		f = "./database.log"
	}

	// Create the file used for logging
	file, err := os.OpenFile(
		f,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		gclog.Fatal("db log file", err)
	}

	// Print separator inside log file
	stat, _ := file.Stat()
	if stat.Size() != 0 {
		// Not the first line of file
		file.WriteString("\n")
	}
	file.WriteString("------ " + time.Now().String() + " ------\n\n")

	// Set the new db logger
	return file
}

// Creates a hub with all channels, caches and database
// Indicates the hub to start running
func setupHub(db *gorm.DB) *Hub {
	// Allocate all data structures
	gormdb := db
	hub := Hub{
		clean:  make(chan net.Conn, gc.MaxClients/2),
		shtdwn: make(chan bool),
		users: table[net.Conn, *User]{
			tab: make(map[net.Conn]*User),
		},
		verifs: table[username, *Verif]{
			tab: make(map[username]*Verif),
		},
		db: gormdb,
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

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	l, err := tls.Listen("tcp4", socket, config)
	if err != nil {
		gclog.Fatal("tls socket setup", err)
	}

	return l
}

// Runs a socket to accept connections
func run(l net.Listener, hub *Hub, count *Counter, wg *sync.WaitGroup) {
	defer wg.Done()

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

func main() {
	// Setup sockets
	sock := setupConn()
	tlssock := setupTLSConn()

	// Set up database logging file
	// Only if logging is INFO or more
	var dblog *log.Logger = nil
	if gclog >= INFO {
		f := logFile()
		defer f.Close()
		dblog = log.New(f, "", log.LstdFlags)
	}

	// Setup database
	db := connectDB(dblog)
	sqldb, _ := db.DB()
	defer sqldb.Close()

	// Setup hub
	hub := setupHub(db)

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for connections! --\n")

	// Endless loop to listen for connections
	var wg sync.WaitGroup
	count := new(Counter)
	wg.Add(2)
	go run(sock, hub, count, &wg)
	go run(tlssock, hub, count, &wg)

	wg.Wait()
}
