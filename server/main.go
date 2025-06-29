package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
)

/* VERSIONING */

// Static version
const serverVersion float32 = 1.1

// Build version
var serverBuild string

// Returns the full version string
func version() string {
	return fmt.Sprintf(
		"%.1f.%s",
		serverVersion,
		serverBuild,
	)
}

/* CONFIG */

// Config struct.
//
// Those values that have a pointer are obligatory fields
type Config struct {
	Database db.Config `json:"database"`
	Server   struct {
		Address *string `json:"address"`
		Port    *uint16 `json:"port"`
		Clients *uint   `json:"max_clients"`
		TLS     struct {
			Enabled     bool    `json:"enabled"`
			Port        *uint16 `json:"port"`
			Certificate *string `json:"cert_file"`
			Key         *string `json:"key_file"`
		} `json:"tls"`
		Logs struct {
			Level string `json:"level"`
			File  string `json:"log_file"`
		} `json:"logs"`
		Motd string `json:"default_motd"`
	} `json:"server"`
}

/* INIT */

// Reads a JSON file for config options
func readJSON(path string) (cfg Config) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal("config file reading", err)
	}
	defer file.Close()

	parser := json.NewDecoder(file)
	parser.Decode(&cfg)

	return cfg
}

// Reads CLI flags and JSON file
//
// setup() should always run first when the program starts
func setup() Config {
	var configFile string
	var useShell bool

	flag.StringVar(&configFile, "config", "config.json", "Configuration file to load, must be in JSON format.")
	flag.BoolVar(&useShell, "shell", false, "Open a database management shell for the server.")
	flag.Parse()

	// Read configuration file
	config := readJSON(configFile)

	if useShell {
		shell := setupShell(config)
		shell.Run()
		os.Exit(0)
	}

	return config
}

/* SETUP FUNCTIONS */

// Sets up the server logs file and level,
// returning the log file to close if necessary
func setupLog(config Config) (file *os.File) {
	file = os.Stdout // Default to stdout
	// Creates a new logging file if it has been specified
	if config.Server.Logs.File != "" {
		// Create the file used for logging
		f, err := os.OpenFile(
			config.Server.Logs.File,
			os.O_RDWR|os.O_CREATE|os.O_APPEND,
			0666,
		)
		if err != nil {
			log.Fatal("db log file", err)
		}
		file = f
	}

	// Set the log output
	stdlog.SetOutput(file)

	// Setup logging levels
	// No need to check if the env var exists
	// We just default to FATAL
	lv := config.Server.Logs.Level
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
	now := time.Now()
	fmt.Printf(
		"-> Logging at %s with log level %s on server version %s and protocol version %d\n",
		now.Format(time.RFC822),
		lv,
		version(),
		spec.ProtocolVersion,
	)

	return file
}

// Creates a database log file and returns it.
func setupDBLog(config Config) (file *os.File) {
	path := config.Database.Logs
	if path == "" {
		path = "./database.log"
	}

	// Create the file used for logging
	f, err := os.OpenFile(
		path,
		os.O_RDWR|os.O_CREATE|os.O_APPEND,
		0666,
	)
	if err != nil {
		log.Fatal("db log file", err)
	}
	file = f

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
func setupConn(config Config) net.Listener {
	addr := config.Server.Address
	if addr == nil {
		log.Config("server.address")
		return nil
	}

	port := config.Server.Port
	if port == nil {
		log.Config("server.port")
		return nil
	}

	socket := fmt.Sprintf(
		"%s:%d",
		*addr,
		*port,
	)

	l, err := net.Listen("tcp", socket)
	if err != nil {
		log.Fatal("socket setup", err)
	}

	log.Notice(fmt.Sprintf("Running TCP Socket on port %d", *port))
	return l
}

// Create a TLS listener
func setupTLSConn(config Config) net.Listener {
	addr := config.Server.Address
	if addr == nil {
		log.Config("server.address")
		return nil
	}

	port := config.Server.TLS.Port
	if port == nil {
		log.Config("server.tls.port")
		return nil
	}

	socket := fmt.Sprintf(
		"%s:%d",
		*addr,
		*port,
	)

	certFile := config.Server.TLS.Certificate
	keyFile := config.Server.TLS.Key
	if certFile == nil {
		log.Config("server.tls.cert_file")
		return nil
	}
	if keyFile == nil {
		log.Config("server.tls.key_file")
		return nil
	}

	cert, err := tls.LoadX509KeyPair(
		*certFile,
		*keyFile,
	)
	if err != nil {
		log.Fatal("tls loading", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	l, err := tls.Listen("tcp", socket, tlsConfig)
	if err != nil {
		log.Fatal("tls socket setup", err)
	}

	log.Notice(fmt.Sprintf("Running TLS Socket on port %d", *port))
	return l
}

/* MAIN FUNCTIONS */

// Specifies a behaviour that is common to all
// listening sockets, so that they can process
// events all at the same time.
type Server struct {
	wg    sync.WaitGroup // How many sockets are running
	count models.Counter // How many clients are connected
}

// Runs a listener to accept connections until the
// shutdown signal is received through the given context.
func (sock *Server) Run(ctx context.Context, l net.Listener, hub *hubs.Hub) {
	defer sock.wg.Done()

	for {
		// This will block until a spot is free
		c, err := l.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				// Server is shutting down
				return
			default:
				log.Error("connection accept", err)
				// Keep accepting clients
				continue
			}
		}

		// Increase and wait if the client counter is full
		sock.count.Inc()

		// Buffered channel for intercommunication between
		// the listening goroutine and the processing goroutine
		req := make(chan hubs.Request, hubs.MaxUserRequests)

		// Listens to the client's packets
		go ListenConnection(
			// We assume no TLS until it passes the handshake
			spec.NewConnection(c, false),
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

	fmt.Printf("-> CTRL-C has been pressed\n")
	log.Notice("manual shutdown signal sent! closing resources...")

	// Send shutdown signal
	close()
}

func main() {
	// Setup config struct
	config := setup()

	// Setup logging (and file optionally)
	logFile := setupLog(config)
	if logFile != nil {
		defer logFile.Close()
	}

	// Set up database logging file only
	// if specified in the config file
	var dblog *stdlog.Logger
	if config.Database.Logging {
		f := setupDBLog(config)
		defer f.Close()
		dblog = stdlog.New(f, "", stdlog.LstdFlags)
	}

	// Setup sockets
	var sock, tlssock net.Listener
	sockets := 1

	sock = setupConn(config)
	if config.Server.TLS.Enabled {
		tlssock = setupTLSConn(config)
		sockets += 1
	}

	// Setup database
	database := db.Connect(dblog, config.Database)
	sqldb, _ := database.DB()
	defer sqldb.Close()

	// Check if max clients has been specified
	if config.Server.Clients == nil {
		log.Config("server.max_clients")
	}

	// Setup hub and make it wait until a shutdown signal is sent
	ctx, cancel := context.WithCancel(context.Background())
	hub := hubs.NewHub(
		database,
		cancel,
		*config.Server.Clients,
		config.Server.Motd,
	)

	if config.Server.TLS.Enabled {
		go hub.Wait(ctx, sock, tlssock)
	} else {
		go hub.Wait(ctx, sock)
	}

	// Just in case a CTRL-C signal happens
	go manual(cancel)

	// Indicate that the server is up and running
	fmt.Printf("-- Server running and listening for connections! --\n")

	// Used for managing all possible sockets
	server := Server{
		count: models.NewCounter(int(*config.Server.Clients)),
	}

	// Endless loop to listen for connections
	server.wg.Add(sockets)
	go server.Run(ctx, sock, hub)
	if config.Server.TLS.Enabled {
		go server.Run(ctx, tlssock, hub)
	}

	// Condition to end program
	server.wg.Wait()
}
