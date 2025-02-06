package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

// Connects to the database using the environment file
func connectDB() *sql.DB {
	user := os.Getenv("DB_USER")
	passwd := os.Getenv("DB_PSWD")
	access := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, passwd, "127.0.1.1:3306", "gochat")

	// Connect to the database
	db, err := sql.Open("mysql", access)
	if err != nil {
		fmt.Println(err)
	}

	// Test that the database works
	e := db.Ping()
	if e != nil {
		fmt.Println(e)
	}

	return db
}
