package db

import (
	"time"

	"gorm.io/gorm"
)

// Returns the highest ID in the specified table.
// This is used to simulate autincremental behaviour in row creation.
func getMaxID(db *gorm.DB, table string) uint {
	var maxID uint
	// If the result of the query is null (the table has no rows) a 0 is returned
	row := db.Raw("SELECT IFNULL(MAX(" + tableToID[table] + "), 0) FROM " + table)
	row.Scan(&maxID)
	return maxID
}

// Adds a user autoincrementally in the database and then returns it.
func addUser(db *gorm.DB, username string, serverID uint) (User, error) {
	user := User{
		UserID:   getMaxID(db, "users") + 1,
		Username: username,
		ServerID: serverID,
	}

	result := db.Create(&user)
	return user, result.Error
}

// Finds a message in the db
func findMessage(db *gorm.DB, srcID, dstID uint, stamp time.Time, text string) (bool, error) {
	var found bool

	result := db.Raw(
		`SELECT EXISTS(
			SELECT *
			FROM messages m
			WHERE m.source_id = ?
				AND m.destination_id = ?
				AND m.stamp = ?
				AND m.text = ?
		) AS found`,
		srcID, dstID, stamp, text,
	).Scan(&found)

	return found, result.Error
}
