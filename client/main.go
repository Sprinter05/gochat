package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// Main method for the client
func main() {
	if len(os.Args) < 2 {
		fmt.Println("error: not enough arguments")
		return
	}
	// Gets .env pathname
	err := godotenv.Load(os.Args[1])
	if err != nil {
		fmt.Println("error: invalid .env path")
		return
	}

	// Connects to the server
	socket := getSocket()
	con, err := net.Dial("tcp4", socket)
	if err != nil {
		log.Fatal(err)
	}
	// Closes conection once execution is over
	defer con.Close()

	// Initializes an empty user
	CurUser = Client{}

	// Opens the client SQLite database
	dbpath, ok := os.LookupEnv("CLT_DB_PATH")
	if !ok {
		fmt.Print("error: variable CLT_DB_PATH not found\n")
		return
	}
	DB, _ = sql.Open("sqlite3", dbpath)
	DeleteEntries() // TODO: remove this

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pctReceived := make(chan struct{})

	// Starts listening for server packets
	go Listen(con, ctx, pctReceived)
	// Opens a shell
	NewShell(con, ctx, pctReceived)
}

func getSocket() string {
	addr, ok := os.LookupEnv("SRV_ADDR")
	if !ok {
		fmt.Print("error: variable SRV_ADDR not found\n")
	}
	port, ok := os.LookupEnv("SRV_PORT")
	if !ok {
		fmt.Print("error: variable SRV_PORT not found\n")
	}
	return fmt.Sprintf("%s:%s", addr, port)
}
