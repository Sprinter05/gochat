package main

import (
	"crypto/rsa"
	"database/sql"
	"fmt"
	"log"
	"os"

	gc "github.com/Sprinter05/gochat/gcspec"
	_ "github.com/go-sql-driver/mysql"
)

/* UTILITIES */

// Connects to the database using the environment file
func connectDB() *sql.DB {
	user := os.Getenv("DB_USER")
	passwd := os.Getenv("DB_PSWD")
	addr := os.Getenv("DB_ADDR")
	port := os.Getenv("DB_PORT")
	dbname := os.Getenv("DB_NAME")
	access := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, passwd, addr, port, dbname)

	// Connect to the database
	db, err := sql.Open("mysql", access)
	if err != nil {
		log.Fatalln(err)
	}

	// Test that the database works
	e := db.Ping()
	if e != nil {
		log.Fatalln(e)
	}

	return db
}

/* QUERIES */

// Lists all usernames in the database
func queryUsernames(db *sql.DB) (string, error) {
	var users string
	query := "SELECT username FROM users;"

	// Try to query all rows
	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	// Iterate over the queried rows
	for rows.Next() {
		// Add the username
		e := rows.Scan(&users)
		if e != nil {
			return "", e
		}
		// Append newline
		users += "\n"
	}

	// Return result without the last newline
	l := len(users)
	return users[:l-1], nil
}

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
			return nil, ErrorDoesNotExist
		}
		return nil, err
	}

	// Check if the user has been deregisterd
	if pubkey == "" {
		return nil, ErrorDeregistered
	}

	// Turn key to rsa struct
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
	query := "INSERT INTO users(username, pubkey) VALUES (?, ?);"

	// Attempt to insert
	_, err := db.Exec(query, uname, string(pubkey))
	if err != nil {
		return err
	}

	return nil
}

// Prevents a user from logging in
func removeKey(db *sql.DB, uname username) error {
	query := "UPDATE users SET pubkey = NULL WHERE username = ?;"

	// Attempt to remove
	_, err := db.Exec(query, uname)
	if err != nil {
		return err
	}

	return nil
}

// Adds a message to the users message cache
func cacheMessage(db *sql.DB, src username, dst username, msg []byte) error {
	userquery := "(SELECT user_id FROM users WHERE username = ?)"
	query := "INSERT INTO message_cache(src_user, dest_user, message) VALUES (" + userquery + ", " + userquery + ", ?);"

	// Attempt to run insert
	_, err := db.Exec(query, src, dst, msg)
	if err != nil {
		return err
	}

	// Everything worked
	return nil
}
