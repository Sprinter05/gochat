package main

import (
	"database/sql"
)

// Adds a user to the database.
func AddUser(username string, pkey string, db *sql.DB) error {
	_, err := db.Exec("INSERT INTO user(username, p_key) VALUES(?, ?)", username, pkey)
	return err
}

// Adds a message to the database.
func AddMessage(source_username string, destination_username string, stamp int64, msg string, db *sql.DB) error {
	id_source_u, _ := getUserID(source_username, db)
	id_destination_u, _ := getUserID(destination_username, db)

	_, err := db.Exec("INSERT INTO message(id_source_u, id_destination_u, unix_stamp, msg) VALUES(?, ?, ?, ?)", id_source_u, id_destination_u, stamp, msg)
	return err
}

// Returns the ID of the specified username, along with the query error (if exists).
// If the query returns an error, result will be 0.
func getUserID(username string, db *sql.DB) (int, error) {
	var result int
	row := db.QueryRow("SELECT id_u FROM user WHERE username = ?", username)
	err := row.Scan(&result)
	return result, err

}

// Returns the ID of the specified username, along with the query error (if exists).
// If the query returns an error, result will be 0.
func GetUserPubkey(username string, db *sql.DB) ([]byte, error) {
	var result string
	row := db.QueryRow("SELECT p_key FROM user WHERE username = ?", username)
	err := row.Scan(&result)
	return []byte(result), err

}

func DeleteEntries(db *sql.DB) error {
	_, userErr := db.Exec("DELETE FROM user")
	if userErr != nil {
		return userErr
	}
	_, msgErr := db.Exec("DELETE FROM message")
	return msgErr
}
