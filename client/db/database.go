package db

// The db package manages the client database connection and holds its interaction functions.
// The database used for the client is SQLite, connected with GORM.

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/* LOGGER */

// Map that allows LogLevel conversions.
var intToLogLevel = map[uint8]logger.LogLevel{
	1: logger.Silent,
	2: logger.Error,
	3: logger.Warn,
	4: logger.Info,
}

// Gets the specified log level in the client configuration file
func GetDBLogger(logLevel uint8, logPath string) logger.Interface {
	dbLogLevel, ok := intToLogLevel[logLevel]
	if !ok {
		log.Fatal("config: unknown log level specified in configuration file")
	}
	// Creates the custom logger
	dbLogFile, ioErr := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	dbLog := logger.New(
		log.New(dbLogFile, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  dbLogLevel,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  false,
		},
	)
	if ioErr != nil {
		log.Fatalf("log file could not be opened: %s", ioErr)
	}
	return dbLog
}

/* ERRORS */

var (
	ErrorInvalidObject error = fmt.Errorf("provided object is not of the correct type")
)

/* CONNECTION */

// Opens the client database.
func OpenDatabase(path string, logger logger.Interface) *gorm.DB {
	clientDB, dbErr := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger})
	if dbErr != nil {
		log.Fatalf("database could not not be opened: %s", dbErr)
	}

	// Makes migrations
	clientDB.AutoMigrate(Server{}, User{}, LocalUser{}, ExternalUser{}, Message{})
	return clientDB
}

/* LOOKUP TABLES */

// Map that contains the ID column of each non-autoincremental table,
// used in getMaxID.
var tableToID = map[string]string{
	"servers": "server_id",
	"users":   "user_id",
}

/* AUXILIARY FUNCTIONS */

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

// Returns a user with an specific id
func getUserByID(db *gorm.DB, id uint) (u User, err error) {
	result := db.Raw(
		`SELECT *
		FROM users
		WHERE user_id = ?`,
		id,
	).Scan(&u)
	return u, result.Error
}

// Finds a message in the database doing a deep search.
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
