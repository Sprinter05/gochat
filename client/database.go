package main

import (
	"database/sql"
)

// Global client database variable.
// ! Variable global mal
// ! La parte de la base de datos la podrias meter en un subpaquete que es mejor
var DB *sql.DB

// Adds a user to the database.
func AddUser(username string, pkey string) error {
	_, err := DB.Exec("INSERT INTO user(username, p_key) VALUES(?, ?)", username, pkey)
	return err
}

// Adds a message to the database.
func AddMessage(source_username string, destination_username string, stamp int64, msg string) error {
	id_source_u, _ := getUserID(source_username)
	id_destination_u, _ := getUserID(destination_username)

	_, err := DB.Exec("INSERT INTO message(id_source_u, id_destination_u, unix_stamp, msg) VALUES(?, ?, ?, ?)", id_source_u, id_destination_u, stamp, msg)
	return err
}

// Returns the ID of the specified username, along with the query error (if exists).
// If the query returns an error, result will be 0.
func getUserID(username string) (int, error) {
	var result int
	row := DB.QueryRow("SELECT id_u FROM user WHERE username = ?", username)
	err := row.Scan(&result)
	return result, err

}

// Returns the ID of the specified username, along with the query error (if exists).
// If the query returns an error, result will be 0.
func GetUserPubkey(username string) ([]byte, error) {
	var result string
	row := DB.QueryRow("SELECT p_key FROM user WHERE username = ?", username)
	err := row.Scan(&result)
	return []byte(result), err

}

func DeleteEntries() error {
	_, userErr := DB.Exec("DELETE FROM user")
	if userErr != nil {
		return userErr
	}
	_, msgErr := DB.Exec("DELETE FROM message")
	return msgErr
}
