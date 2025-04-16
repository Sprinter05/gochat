package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/joho/godotenv"
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

	data := ShellData{ClientCon: cl, Verbose: true}
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
