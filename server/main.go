package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/hubs"

	"github.com/joho/godotenv"
)

// CLI Flags
var (
	envFile string
	useTLS  bool
)

/* INIT */

// Sets up logging and reads cli flags
//
// init() always runs first when the program starts
func init() {
	flag.StringVar(&envFile, "env", "", "Environment file to load")
	flag.BoolVar(&useTLS, "tls", true, "Whether to use a TLS socket or not")

	// If we default to stderr it won't print unless debugged
	stdlog.SetOutput(os.Stdout)

	flag.Parse()

	// Read argument 1 as .env if it exists
	if envFile != "" {
		err := godotenv.Load(envFile)
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

// Creates a log file and returns it.
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

	// Prints that the server has started
	// running inside log file
	stat, _ := file.Stat()
	if stat.Size() != 0 {
		// Not the first line of file
		file.WriteString("\n")
	}
	file.WriteString("------ " + time.Now().String() + " ------\n\n")

	return file
}

// Creates an unencrypted TCP listener
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

	log.Notice(fmt.Sprintf("Running TCP Socket on port %s", port))
	return l
}

// Create a TLS listener
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

	log.Notice(fmt.Sprintf("Running TLS Socket on port %s", port))
	return l
}

/* MAIN FUNCTIONS */

// TODO: Server intranet writeup

// Specifies a behaviour that is common to all
// listening sockets, so that they can process
// events all at the same time.
type Server struct {
	wg    sync.WaitGroup  // How many sockets are running
	count models.Counter  // How many clients are connected
	ctx   context.Context // When to shut down the socket
}

// Runs a listener to accept connections until the
// shutdown signal is received.
func (sock *Server) Run(l net.Listener, hub *hubs.Hub) {
	defer sock.wg.Done()

	for {
		// This will block until a spot is free
		c, err := l.Accept()
		if err != nil {
			select {
			case <-sock.ctx.Done():
				// Server is shutting down
				return
			default:
				log.Error("connection accept", err)
				// Keep accepting clients
				continue
			}
		}
		sock.count.Inc()

		// Set timeout for the initial write to prevent blocking new connections
		deadline := time.Now().Add(time.Duration(spec.HandshakeTimeout) * time.Minute)
		c.SetDeadline(deadline)

		// Notify the user they are connected
		pak, e := spec.NewPacket(spec.OK, spec.NullID, spec.EmptyInfo)
		if e != nil {
			log.Packet(spec.OK, e)
		} else {
			_, err := c.Write(pak)
			if err != nil {
				log.Error("handshake with new connection", err)
			}
		}

		// Disable timeout as it is only for the first write
		c.SetDeadline(time.Time{})

		// Check if its tls
		_, ok := c.(*tls.Conn)

		// Buffered channel for intercommunication between
		// the listening goroutine and the processing goroutine
		req := make(chan hubs.Request, hubs.MaxUserRequests)

		// Listens to the client's packets
		go ListenConnection(
			spec.NewConnection(c, ok),
			&sock.count,
			req,
			hub,
		)

		// Runs the client's commands
		go RunTask(hub, req)
	}
}

// Waits on a CTRL-C signal by the OS
func manual(close context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Wait for a CTRL-C
	<-c

	log.Notice("manual shutdown signal sent! closing resources...")

	// Send shutdown signal
	close()
}

func main() {
	// Setup sockets
	var sock, tlssock net.Listener
	sockets := 1

	sock = setupConn()
	if useTLS {
		tlssock = setupTLSConn()
		sockets += 1
	}

	// Set up database logging file only
	// if the logging level is INFO or more
	var dblog *stdlog.Logger
	if log.Level >= log.INFO {
		f := logFile()
		defer f.Close()
		dblog = stdlog.New(f, "", stdlog.LstdFlags)
	}

	// Setup database
	database := db.Connect(dblog)
	sqldb, _ := database.DB()
	defer sqldb.Close()

	// Setup hub and make it wait until a shutdown signal is sent
	ctx, cancel := context.WithCancel(context.Background())
	hub := hubs.NewHub(database, ctx, cancel, spec.MaxClients)

	if useTLS {
		go hub.Wait(sock, tlssock)
	} else {
		go hub.Wait(sock)
	}

	// Just in case a CTRL-C signal happens
	go manual(cancel)

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for connections! --\n")

	// Used for managing all possible sockets
	server := Server{
		ctx:   ctx,
		count: models.NewCounter(int(spec.MaxClients)),
	}

	// Endless loop to listen for connections
	server.wg.Add(sockets)
	go server.Run(sock, hub)
	if useTLS {
		go server.Run(tlssock, hub)
	}

	// Condition to end program
	server.wg.Wait()
}
