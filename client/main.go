package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var intToLogLevel = map[uint8]logger.LogLevel{
	1: logger.Silent,
	2: logger.Error,
	3: logger.Warn,
	4: logger.Info,
}

// Stores the json attributes of the client configuration file
type Config struct {
	Server struct {
		Address string `json:"address"`
		Port    string `json:"port"`
	} `json:"server"`
	Database struct {
		Path     string `json:"path"`
		LogPath  string `json:"log_path"`
		LogLevel uint8  `json:"log_level"`
	} `json:"database"`
}

// Main client function
func main() {
	// Reads configuration file
	config := getConfig()
	// Connects to the server
	socket := net.JoinHostPort(config.Server.Address, config.Server.Port)
	con, conErr := net.Dial("tcp4", socket)
	if conErr != nil {
		log.Fatalf("could not establish a TCP connection with server: %s", conErr)
	}
	cl := spec.Connection{Conn: con}
	defer con.Close() // Closes conection right before execution ends

	// Gets the specified log level
	dbLogLevel, ok := intToLogLevel[config.Database.LogLevel]
	if !ok {
		log.Fatal("config: unknown log level specified in configuration file")
	}
	fmt.Println(dbLogLevel)
	// Creates the custom logger
	dbLogFile, ioErr := os.OpenFile(config.Database.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	dbLog := logger.New(
		log.New(dbLogFile, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  dbLogLevel,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	if ioErr != nil {
		log.Fatalf("log file could not be opened: %s", ioErr)
	}
	// Opens the client database
	db, dbErr := gorm.Open(sqlite.Open("client/db/client.db"), &gorm.Config{Logger: dbLog})
	if dbErr != nil {
		log.Fatalf("database could not not be opened: %s", dbErr)
	}

	data := ShellData{ClientCon: cl, Verbose: true, DB: db}
	ConnectionStart(&data)

	go Listen(&data)
	NewShell(&data)
}

// Returns a Config struct with the data obtained from the json
// configuration file.
func getConfig() Config {
	config := Config{}

	f, err := os.Open("client_config.json")
	if err != nil {
		log.Fatalf("configuration file could not be opened: %s", err)
	}
	defer f.Close()

	jsonParser := json.NewDecoder(f)
	jsonParser.Decode(&config)
	return config
}
