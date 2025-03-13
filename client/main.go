package main

import (
	"database/sql"
	"log"
	"net"
	_ "github.com/mattn/go-sqlite3"
)

// Main method for the client

func main() {
	// Connects to the server
	con, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		log.Fatal(err)
	}
	// Closes conection once execution is over
	defer con.Close()
	dbpath, ok := os.LookupEnv("CLT_DB_PATH")
	if !ok {
		fmt.Print("error: variable CLT_DB_PATH not found\n")
		return
	}
	DB, _ = sql.Open("sqlite3", dbpath)
	DeleteEntries() // TODO: remove this

	// Starts listening for server packets
	go Listen(con, ctx, pctReceived)
	// Opens a shell
	NewShell(con, ctx, pctReceived)
}
