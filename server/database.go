package main

import (
	"crypto/rsa"
	"database/sql"
	"fmt"
	"os"

	gc "github.com/Sprinter05/gochat/gcspec"
	_ "github.com/go-sql-driver/mysql"
)

/* UTILITIES */

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

/* QUERIES */

// Retrieves the user public key if it exists wih a nil net.Conn
func queryUserKey(db *sql.DB, uname username) (*rsa.PublicKey, error) {
	var pubkey string
	query := "SELECT pubkey FROM users WHERE username = ?;"

	// Query database
	row := db.QueryRow(query, string(uname))
	err := row.Scan(&pubkey)
	if err != nil {
		if err == sql.ErrNoRows {
			// User does not exist
			return nil, gc.ErrorNotFound
		}
		return nil, err
	}

	// Turn key to rsa struct
	//! If this fails it means the user database has invalid information
	key, err := gc.PEMToPubkey([]byte(pubkey))
	if err != nil {
		return nil, err
	}

	// Return the key
	return key, nil
}

/* INSERTIONS */

// Inserts a user into a database, key must be in PEM format
func insertUser(db *sql.DB, uname username, pubkey []byte) error {
	query := "INSERT INTO users(username, pubkey) VALUES (?, ?)"

	// Attempt to insert
	_, err := db.Exec(query, uname, string(pubkey))
	if err != nil {
		return err
	}

	return nil
}
