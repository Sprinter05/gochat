package main

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"time"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
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

	// Opens the database
	dbLog := getDBLogger(config)
	clientDB := openClientDatabase(config.Database.Path, dbLog)

	var cl spec.Connection
	var con net.Conn
	if address != "" {
		var conErr error
		con, conErr = commands.Connect(address, port)
		if conErr != nil {
			log.Fatal(conErr)
		}
	}
	cl = spec.Connection{Conn: con}

	server := db.SaveServer(clientDB, address, port)
	// TODO: verbose to config
	data := commands.Data{ClientCon: cl, Verbose: true, Output: ShellPrint, DB: clientDB, Server: server}

	if address != "" {
		commands.ConnectionStart(data, ShellPrint)
	}

	// go Listen(&data)
	NewShell(&data)
	if data.ClientCon.Conn != nil {
		data.ClientCon.Conn.Close()
	}
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

// Gets the specified log level in the client configuration file
func getDBLogger(config Config) logger.Interface {
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
func openClientDatabase(path string, logger logger.Interface) *gorm.DB {
	clientDB, dbErr := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger})
	if dbErr != nil {
		log.Fatalf("database could not not be opened: %s", dbErr)
	}

	// Makes migrations
	clientDB.AutoMigrate(&db.Server{}, &db.User{}, &db.LocalUserData{}, &db.ExternalUserData{}, &db.Message{})
	return clientDB
}
