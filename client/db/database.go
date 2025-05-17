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

// Map that contains the ID column of each non-autoincremental table, used in getMaxID.
var tableToID = map[string]string{
	"servers": "server_id",
	"users":   "user_id",
}

// Map that allows LogLevel conversions.
var intToLogLevel = map[uint8]logger.LogLevel{
	1: logger.Silent,
	2: logger.Error,
	3: logger.Warn,
	4: logger.Info,
}

// Generic user table that defines the columns every user shares.
type User struct {
	UserID   uint   `gorm:"autoIncrement:false;not null"`
	ServerID uint   `gorm:"primaryKey;autoIncrement:false;not null"`
	Username string `gorm:"primaryKey;not null"`
	Server   Server `gorm:"foreignKey:ServerID;references:ServerID;constraint:OnDelete:CASCADE"`
}

// User extension dedicated to shell-created users.
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
	ServerID uint   `gorm:"autoIncrement:false;not null"`
	Name     string `gorm:"unique;not null"`
}

var ErrorUnexpectedRows error = fmt.Errorf("unexpected number of rows affected in User creation")

// Opens the client database.
func OpenClientDatabase(path string, logger logger.Interface) *gorm.DB {
	clientDB, dbErr := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger})
	if dbErr != nil {
		log.Fatalf("database could not not be opened: %s", dbErr)
	}

	// Makes migrations
	clientDB.AutoMigrate(Server{}, User{}, LocalUser{}, ExternalUser{}, Message{})
	return clientDB
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

// Returns the highest ID in the specified table.
// This is used to simulate autincremental behaviour in row creation.
func getMaxID(db *gorm.DB, table string) uint {
	var maxID uint
	// If the result of the query is null (the table has no rows) a 0 is returned
	row := db.Raw("SELECT IFNULL(MAX(" + tableToID[table] + "), 0) FROM " + table)
	row.Scan(&maxID)
	return maxID
}

// Adds a socket pair to the database if the socket is not on it already. Then,
// returns it.
func SaveServer(db *gorm.DB, address string, port uint16, name string) (Server, error) {
	// Adds the server to the database only if it is not in it already
	server := Server{ServerID: getMaxID(db, "servers") + 1, Address: address, Port: port, Name: name}
	var saveError error

	svExists, existsErr := ServerExists(db, address, port)
	if existsErr != nil {
		return Server{}, existsErr
	}

	if !svExists {
		_, err := AddServer(db, server)
		if err != nil {
			return server, err
		}
	} else {
		newServer, getErr := GetServer(db, address, port)
		if getErr != nil {
			return Server{}, getErr
		}
		server.ServerID = newServer.ServerID
		server.Name = name
		result := db.Save(&server)
		saveError = result.Error
	}
	return server, saveError

}

// Creates a server, then returns it.
func AddServer(db *gorm.DB, server Server) (Server, error) {
	result := db.Create(&server)

	if result.RowsAffected != 1 {
		return Server{}, ErrorUnexpectedRows
	}
	return server, result.Error
}

// Deletes a server from the database.
func RemoveServer(db *gorm.DB, address string, port uint16) error {
	server := Server{Address: address, Port: port}
	result := db.Delete(&server)
	if result.RowsAffected != 1 {
		return ErrorUnexpectedRows
	}
	return result.Error
}

// Returns the server with the specified socket.
func GetServer(db *gorm.DB, address string, port uint16) (Server, error) {
	var server Server
	result := db.Where("address = ? AND port = ?", address, port).First(&server)
	return server, result.Error
}

// Returns all servers
func GetAllServers(db *gorm.DB) ([]Server, error) {
	var servers []Server
	result := db.Raw("SELECT * FROM servers").Scan(&servers)
	return servers, result.Error
}

// Returns true if the specified socket exists in the database.
func ServerExists(db *gorm.DB, address string, port uint16) (bool, error) {
	var found bool = false
	result := db.Raw("SELECT EXISTS(SELECT * FROM servers WHERE address = ? AND port = ?) AS found", address, port).Scan(&found)
	return found, result.Error
}

// Returns true if the specified username and server defines a local user in the database.
func LocalUserExists(db *gorm.DB, username string) (bool, error) {
	var found bool = false
	result := db.Raw("SELECT EXISTS(SELECT * FROM users, local_users WHERE users.user_id = local_users.user_id AND username = ?) AS found", username).Scan(&found)
	return found, result.Error
}

// Returns the user that is defined by the username and server.
func GetUser(db *gorm.DB, username string, serverID uint) (User, error) {
	user := User{Username: username, ServerID: serverID}
	result := db.First(&user)
	return user, result.Error
}

// Returns the local user that is defined by the specified username and server.
func GetLocalUser(db *gorm.DB, username string, serverID uint) (LocalUser, error) {
	user, err := GetUser(db, username, serverID)
	if err != nil {
		return LocalUser{}, err
	}

	localUser := LocalUser{User: User{Username: username}, UserID: user.UserID}
	result := db.First(&localUser)
	return localUser, result.Error
}

// Gets all the local usernames (used in USRS to print local usernames).
func GetAllLocalUsernames(db *gorm.DB) ([]string, error) {
	var usernames []string
	result := db.Raw("SELECT username FROM users, local_users WHERE local_users.user_id = users.user_id").Scan(&usernames)
	return usernames, result.Error
}

// Adds a user autoincrementally in the database and then returns it.
func addUser(db *gorm.DB, username string, serverID uint) (User, error) {
	user := User{UserID: getMaxID(db, "users") + 1, Username: username, ServerID: serverID}
	result := db.Create(&user)
	if result.RowsAffected != 1 {
		return User{}, ErrorUnexpectedRows
	}
	return user, result.Error
}

// Adds a local user autoincrementally in the database and then returns it.
func AddLocalUser(db *gorm.DB, username string, hashPass string, prvKeyPEM string, serverID uint) (LocalUser, error) {
	// Attempts to create the user. If there's a user with that username and server already
	// the local user will not be created
	user, userErr := addUser(db, username, serverID)
	if userErr != nil {
		return LocalUser{}, userErr
	}
	localUser := LocalUser{User: user, UserID: user.UserID, Password: hashPass, PrvKey: prvKeyPEM}

	result := db.Create(&localUser)
	if result.RowsAffected != 1 {
		return localUser, ErrorUnexpectedRows
	}
	return localUser, result.Error
}

// Adds a local user autoincrementally in the database and then returns it.
func AddExternalUser(db *gorm.DB, username string, pubKeyPEM string, serverID uint) (ExternalUser, error) {
	// Attempts to create the user. If there's a user with that username and server already
	// the local user will not be created
	user, userErr := addUser(db, username, serverID)
	if userErr != nil {
		return ExternalUser{}, userErr
	}
	externalUser := ExternalUser{User: user, UserID: user.UserID, PubKey: pubKeyPEM}
	result := db.Create(&externalUser)
	if result.RowsAffected != 1 {
		return ExternalUser{}, ErrorUnexpectedRows
	}
	return externalUser, result.Error
}

// Returns the external user that is defined by the specified username and server.
func GetExternalUser(db *gorm.DB, username string, serverID uint) (ExternalUser, error) {
	user, err := GetUser(db, username, serverID)
	if err != nil {
		return ExternalUser{}, err
	}
	externalUser := ExternalUser{User: User{Username: username}, UserID: user.UserID}
	result := db.First(&externalUser)
	return externalUser, result.Error
}

// Returns true if the specified username and server defines an external user in the database.
func ExternalUserExists(db *gorm.DB, username string) (bool, error) {
	var found bool = false
	result := db.Raw("SELECT EXISTS(SELECT * FROM users, external_users WHERE users.user_id = external_users.user_id AND username = ?) AS found", username).Scan(&found)
	return found, result.Error
}

// Adds a message to the database and returns it.
func StoreMessage(db *gorm.DB, src User, dst User, text string, stamp time.Time) (Message, error) {
	msg := Message{SourceID: src.UserID, DestinationID: dst.UserID, Text: text, Stamp: stamp}
	result := db.Create(&msg)

	if result.RowsAffected != 1 {
		return Message{}, ErrorUnexpectedRows
	}
	return msg, result.Error
}

// Returns a slice with every message between two users in a range of time
func GetUsersMessagesRange(db *gorm.DB, src User, dst User, init time.Time, end time.Time) ([]Message, error) {
	var messages []Message
	result := db.Where("stamp BETWEEN ? AND ?", init, end).Where("(source_id = ? AND destination_id = ?) OR (source_id = ? AND destination_id = ?)", src.UserID, dst.UserID, dst.UserID, src.UserID).Find(&messages)
	if result.Error != nil {
		return nil, result.Error
	}
	return messages, nil
}

// Returns a slice with all messages between two users
func GetAllUsersMessages(db *gorm.DB, src User, dst User) ([]Message, error) {
	var messages []Message
	result := db.Where("(source_id = ? AND destination_id = ?) OR (source_id = ? AND destination_id = ?)", src.UserID, dst.UserID, dst.UserID, src.UserID).Find(&messages)
	if result.Error != nil {
		return nil, result.Error
	}
	return messages, nil
}
