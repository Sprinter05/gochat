package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/hubs"
	"github.com/Sprinter05/gochat/server/model"

	"github.com/joho/godotenv"
)

/* INIT */

// Sets up logging
// Reads environment file from first cli argument
// init() always runs when the program starts
func init() {
	// If we default to stderr it won't print unless debugged
	stdlog.SetOutput(os.Stdout)

	// Argument 0 is the pathname to the executable
	if len(os.Args) > 1 {
		// Read argument 1 as .env if it exists
		err := godotenv.Load(os.Args[1])
		if err != nil {
			log.Fatal("env file reading", err)
		}
	}

	// Setup logging levels
	// No need to check if the env var exists
	// We just default to FATAL
	lv := os.Getenv("LOG_LEVL")
	switch lv {
	case "ALL":
		log.Level = log.ALL
	case "INFO":
		log.Level = log.INFO
	case "ERROR":
		log.Level = log.ERROR
	default:
		log.Level = log.FATAL
		lv = "FATAL"
	}
	fmt.Printf("-> Logging with log level %s\n", lv)
}

/* SETUP FUNCTIONS */

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
		log.Fatal("db log file", err)
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

// Creates a listener for the socket
func setupConn() net.Listener {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		log.Environ("SRV_ADDR")
	}

	port, ok := os.LookupEnv("SRV_PORT")
	if !ok {
		log.Environ("SRV_PORT")
	}

	socket := fmt.Sprintf(
		"%s:%s",
		addr,
		port,
	)

	l, err := net.Listen("tcp4", socket)
	if err != nil {
		log.Fatal("socket setup", err)
	}

	return l
}

// Create a TLS listener for the socket
func setupTLSConn() net.Listener {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		log.Environ("SRV_ADDR")
	}

	port, ok := os.LookupEnv("TLS_PORT")
	if !ok {
		log.Environ("TLS_PORT")
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
		log.Fatal("tls loading", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	l, err := tls.Listen("tcp4", socket, config)
	if err != nil {
		log.Fatal("tls socket setup", err)
	}

	return l
}

/* MAIN FUNCTIONS */

// TODO: Document everything (100go.co #15)
// TODO: Pass linter and formatter
//? TODO: Accept UTF-8 encoding

// Identifies a running socket
type Socket struct {
	wg    *sync.WaitGroup
	count *model.Counter
	ctx   context.Context
}

// Runs a socket to accept connections
func Run(l net.Listener, hub *hubs.Hub, sock *Socket) {
	defer sock.wg.Done()

	for {
		select {
		case <-sock.ctx.Done():
			return
		default:
			// This will block until a spot is free
			c, err := l.Accept()
			if err != nil {
				log.Error("connection accept", err)
				// Keep accepting clients
				continue
			}
			sock.count.Inc()

			// Notify the user they are connected
			pak, e := spec.NewPacket(spec.OK, spec.NullID, spec.EmptyInfo)
			if e != nil {
				log.Packet(spec.OK, e)
			} else {
				c.Write(pak)
			}

			// Check if its tls
			_, ok := c.(*tls.Conn)

			cl := spec.Connection{
				Conn: c,
				RD:   bufio.NewReader(c),
				TLS:  ok,
			}

			// Buffered channel for intercommunication
			req := make(chan hubs.Request, hubs.MaxUserRequests)

			// Listens to the client's packets
			go ListenConnection(cl, sock.count, req, hub)

			// Runs the client's commands
			go RunTask(hub, req)
		}
	}
}

func main() {
	// Setup sockets
	sock := setupConn()
	tlssock := setupTLSConn()

	// Set up database logging file
	// Only if logging is INFO or more
	var dblog *stdlog.Logger = nil
	if log.Level >= log.INFO {
		f := logFile()
		defer f.Close()
		dblog = stdlog.New(f, "", stdlog.LstdFlags)
	}

	// Setup database
	database := db.Connect(dblog)
	sqldb, _ := database.DB()
	defer sqldb.Close()

	// Setup hub
	ctx, cancel := context.WithCancel(context.Background())
	hub := hubs.Create(database, cancel)

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for connections! --\n")

	// We wait for both sockets to end
	var wg sync.WaitGroup
	count := model.NewCounter(spec.MaxClients)
	wg.Add(2)

	// Used for safely cleaning up resrouces
	comm := Socket{
		wg:    &wg,
		ctx:   ctx,
		count: &count,
	}

	// Endless loop to listen for connections
	go Run(sock, hub, &comm)
	go Run(tlssock, hub, &comm)

	// Condition to end program
	wg.Wait()
}
