package db

// The db package manages the client database connection and holds its interaction functions.
// The database used for the client is SQLite, connected with GORM.

import (
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

/* MODELS */

// Generic user table that defines the columns every user shares.
type User struct {
	UserID   uint   `gorm:"autoIncrement:false;not null"`
	ServerID uint   `gorm:"primaryKey;autoIncrement:false;not null"`
	Username string `gorm:"primaryKey;not null"`
	Server   Server `gorm:"foreignKey:ServerID;references:ServerID;constraint:OnDelete:CASCADE"`
}

// User extension dedicated to locally created users.
// The passwords should be hashed and the private
// keys need to be stored in PEM format.
type LocalUser struct {
	UserID   uint   `gorm:"primaryKey;not null;autoIncrement:false"`
	Password string `gorm:"not null"`
	PrvKey   string
	User     User `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// User extension dedicated to REQ'd users. Only
// their public key is needed to encrypt messages
// to them.
type ExternalUser struct {
	UserID uint   `gorm:"primaryKey;not null"`
	PubKey string `gorm:"not null"`
	User   User   `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// Holds message data.
type Message struct {
	MessageID       uint `gorm:"primaryKey;autoincrement;not null"`
	SourceID        uint
	DestinationID   uint
	Stamp           time.Time
	Text            string
	SourceUser      User `gorm:"foreignKey:SourceID;references:UserID;OnDelete:RESTRICT"`
	DestinationUser User `gorm:"foreignKey:DestinationID;references:UserID;OnDelete:RESTRICT"`
}

// Server indentifier that allows a multi-server platform.
type Server struct {
	Address  string `gorm:"primaryKey;autoIncrement:false;not null"`
	Port     uint16 `gorm:"primaryKey;autoIncrement:false;not null"`
	TLS      bool   `gorm:"not null"`
	ServerID uint   `gorm:"autoIncrement:false;not null"`
	Name     string `gorm:"unique;not null"`
}

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
