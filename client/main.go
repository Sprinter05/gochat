package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"strconv"
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
		Port    uint16 `json:"port"`
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

	address := config.Server.Address
	port := config.Server.Port
	con := Connect(address, port)
	cl := spec.Connection{Conn: con}
	defer con.Close() // Closes conection right before execution ends

	dbLog := GetDBLogger(config)
	db := OpenClientDatabase(config.Database.Path, dbLog)

	server := SaveServer(db, address, port)
	data := ShellData{ClientCon: cl, Verbose: true, DB: db, Server: server}

	// Fills the Server related data in the ShellData struct
	ConnectionStart(data)

	// go Listen(&data)
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

// Connects to the gochat server given its address and port
func Connect(address string, port uint16) net.Conn {
	socket := net.JoinHostPort(address, strconv.FormatUint(uint64(port), 10))
	con, conErr := net.Dial("tcp4", socket)
	if conErr != nil {
		log.Fatalf("could not establish a TCP connection with server: %s", conErr)
	}
	return con
}

// Gets the specified log level in the client configuration file
func GetDBLogger(config Config) logger.Interface {
	dbLogLevel, ok := intToLogLevel[config.Database.LogLevel]
	if !ok {
		log.Fatal("config: unknown log level specified in configuration file")
	}
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
	return dbLog
}

// Opens the client database
func OpenClientDatabase(path string, logger logger.Interface) *gorm.DB {
	db, dbErr := gorm.Open(sqlite.Open("client/db/client.db"), &gorm.Config{Logger: logger})
	if dbErr != nil {
		log.Fatalf("database could not not be opened: %s", dbErr)
	}

	// Makes migrations
	db.AutoMigrate(&Server{}, &User{}, &LocalUserData{}, &ExternalUserData{}, &Message{})
	return db
}
