package main

import (
	"crypto/rsa"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	gc "github.com/Sprinter05/gochat/gcspec"
	myqsl "github.com/go-sql-driver/mysql"
)

/* UTILITIES */

// Gets the environment variables necessary
func getDBEnv() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		gclog.Env("DB_USER")
	}

	pswd, ok := os.LookupEnv("DB_PSWD")
	if !ok {
		gclog.Env("DB_PSWD")
	}

	addr, ok := os.LookupEnv("DB_ADDR")
	if !ok {
		gclog.Env("DB_ADDR")
	}

	port, ok := os.LookupEnv("DB_PORT")
	if !ok {
		gclog.Env("DB_PORT")
	}

	name, ok := os.LookupEnv("DB_NAME")
	if !ok {
		gclog.Env("DB_NAME")
	}

	// Get formatted string
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s",
		user,
		pswd,
		addr,
		port,
		name,
	)
}

// Connects to the database using the environment file
func connectDB() *sql.DB {
	access := getDBEnv()
	db, err := sql.Open("mysql", access)
	if err != nil {
		gclog.Fatal("database login", err)
	}

	// Test that the database works
	e := db.Ping()
	if e != nil {
		gclog.Fatal("database ping", err)
	}

	return db
}

/* QUERIES */

// Queries the amount of messages cached for that user
func queryMessageQuantity(db *sql.DB, uname username) (int, error) {
	var size int
	query := `
		SELECT COUNT(msg) 
		FROM message_cache mc 
		JOIN users u ON mc.src_user = u.user_id 
		WHERE mc.dest_user = (
			SELECT user_id 
			FROM users 
			WHERE username = ?
		);
	`

	row := db.QueryRow(query, string(uname))
	err := row.Scan(&size)
	if err != nil {
		return -1, err
	}

	return size, nil
}

// Gets all messages from the user
// It is expected for the size to be queried previously
func queryMessages(db *sql.DB, uname username, size int) (*[]Message, error) {
	query := `
		SELECT username, msg, UNIX_TIMESTAMP(stamp) 
		FROM message_cache mc JOIN users u ON mc.src_user = u.user_id 
		WHERE mc.dest_user = (
			SELECT user_id 
			FROM users 
			WHERE username = ?
		) 
		ORDER BY stamp ASC;
	`

	rows, err := db.Query(query, uname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// We allocate for the necessary messages
	message := make([]Message, size)

	for i := 0; rows.Next(); i++ {
		var temp string
		err := rows.Scan(
			&message[i].sender,
			&temp,
			&message[i].stamp,
		)

		// Conversion from hex string
		dec, _ := hex.DecodeString(temp)
		message[i].message = string(dec)

		if err != nil {
			return nil, err
		}
	}

	return &message, nil
}

// Lists all usernames in the database
func queryUsernames(db *sql.DB) (string, error) {
	var users strings.Builder
	query := `
		SELECT username 
		FROM users;
	`

	rows, err := db.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var temp string
		// Without temp variable it'd be overwritten
		e := rows.Scan(&temp)
		if e != nil {
			return "", e
		}
		// Append to buffer
		users.WriteString(temp + "\n")
	}

	// Return result without the last newline
	l := users.Len()
	slice := users.String()
	return slice[:l-1], nil
}

// Retrieves the user public key if it exists wih a nil net.Conn
func queryUserKey(db *sql.DB, uname username) (*rsa.PublicKey, error) {
	var pubkey sql.NullString
	query := `
		SELECT pubkey 
		FROM users 
		WHERE username = ?;
	`

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
	if !pubkey.Valid {
		return nil, ErrorDeregistered
	}

	key, err := gc.PEMToPubkey([]byte(pubkey.String))
	if err != nil {
		return nil, err
	}

	return key, nil
}

// Returns the privilege level of the user
func queryUserPerms(db *sql.DB, uname username) (Permission, error) {
	var perms Permission
	query := `
		SELECT permission
		FROM users 
		WHERE username = ?;
	`

	row := db.QueryRow(query, string(uname))
	err := row.Scan(&perms)
	if err != nil {
		if err == sql.ErrNoRows {
			// User does not exist
			return -1, ErrorDoesNotExist
		}
		return -1, err
	}

	if perms > OWNER {
		// Already max permissions
		return -1, ErrorInvalidValue
	}

	return perms, nil
}

/* INSERTIONS AND UPDATES */

// Inserts a user into a database, key must be in PEM format
func insertUser(db *sql.DB, uname username, pubkey []byte) error {
	query := `
		INSERT INTO users(username, pubkey) 
		VALUES (?, ?);
	`

	_, err := db.Exec(query, uname, string(pubkey))
	if err != nil {
		return err
	}

	return nil
}

// Adds a message to the users message cache
// The message must be in byte array format since its encrypted
func cacheMessage(db *sql.DB, src username, dst username, msg []byte) error {
	query := `
		INSERT INTO message_cache(src_user, dest_user, msg) 
		VALUES (
			(
				SELECT user_id 
				FROM users 
				WHERE username = ?
			), 
			(
				SELECT user_id 
				FROM users 
				WHERE username = ?
			), 
		?);
	`
	str := hex.EncodeToString(msg)

	_, err := db.Exec(query, src, dst, str)
	if err != nil {
		return err
	}

	return nil
}

// Prevents a user from logging in
func removeKey(db *sql.DB, uname username) error {
	query := `
		UPDATE users 
		SET pubkey = NULL 
		WHERE username = ?;
	`

	_, err := db.Exec(query, uname)
	if err != nil {
		return err
	}

	return nil
}

// Changes the permissions of a user
func changePermissions(db *sql.DB, uname username, perm Permission) error {
	query := `
		UPDATE users
		SET permission = ?
		WHERE username = ?;
	`

	_, err := db.Exec(query, perm, uname)
	if err != nil {
		return err
	}

	return nil
}

/* DELETIONS */

// Removes a user from the database
func removeUser(db *sql.DB, uname username) error {
	query := `
		DELETE FROM users 
		WHERE username = ?
	`

	_, err := db.Exec(query, uname)
	if err != nil {
		// Unwrap error as driver error
		var sqlerr *myqsl.MySQLError
		ok := errors.As(err, &sqlerr)
		// Check if the error is the foreign key constraint
		if ok && sqlerr.Number == 1451 {
			return ErrorDBConstraint
		}
		return err
	}

	return nil
}

// Removes all cached messages from a user before a given stamp
// This is done to prevent messages from being lost
func removeMessages(db *sql.DB, uname username, stamp int64) error {
	query := `
		DELETE FROM message_cache 
		WHERE dest_user = (
			SELECT user_id 
			FROM users 
			WHERE username = ?
		) AND stamp <= FROM_UNIXTIME(?);
	`

	_, err := db.Exec(query, uname, stamp)
	if err != nil {
		return err
	}

	return nil
}
