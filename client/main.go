package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/joho/godotenv"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Main client function
func main() {
	if len(os.Args) < 2 {
		log.Fatal("error: not enough arguments")
		return
	}
	// Gets .env pathname
	err := godotenv.Load(os.Args[1])
	if err != nil {
		log.Fatal("error: invalid .env path")
		return
	}
	// Connects to the server
	socket := getSocket()
	con, err := net.Dial("tcp4", socket)
	if err != nil {
		log.Fatal(err)
	}
	cl := spec.Connection{Conn: con}
	defer con.Close() // Closes conection once execution is over

	// Creates the database custom logger
	// TODO: config file
	logFile, ioErr := os.OpenFile("logs/client_db.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	dbLog := logger.New(
		log.New(logFile, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	if ioErr != nil {
		log.Fatal(ioErr)
	}

	db, dbErr := gorm.Open(sqlite.Open("client/db/client.db"), &gorm.Config{Logger: dbLog})
	if dbErr != nil {
		log.Fatal(dbErr)
	}

	data := ShellData{ClientCon: cl, Verbose: true, DB: db}
	ConnectionStart(&data)

	go Listen(&data)
	NewShell(&data)
}

// Returns a string with the appropiate format to define a socket
func getSocket() string {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		log.Fatal("error: variable SRV_ADDR not found\n")
	}
	port, ok := os.LookupEnv("SRV_PORT")
	if !ok {
		log.Fatal("error: variable SRV_PORT not found\n")
	}
	return fmt.Sprintf("%s:%s", addr, port)
}
