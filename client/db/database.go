package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Map that contains the ID column of each non-autoincremental table, used in getMaxID
var tableToID = map[string]string{
	"servers": "server_id",
	"users":   "user_id",
}

// Generic user table that defines the columns every user shares
type User struct {
	UserID   uint   `gorm:"autoIncrement:false;not null"`
	ServerID uint   `gorm:"primaryKey;autoIncrement:false;not null"`
	Username string `gorm:"primaryKey;not null"`
	Server   Server `gorm:"foreignKey:ServerID;references:ServerID;constraint:OnDelete:CASCADE"`
}

// User extension dedicated to shell-created users.
// The passwords should be hashed and the private
// keys need to be stored
type LocalUserData struct {
	UserID   uint   `gorm:"primaryKey;not null;autoIncrement:false"`
	Password string `gorm:"not null"`
	PrvKey   string
	User     User `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// User extension dedicated to REQ'd users. Only
// their public key is needed to encrypt messages
// to them
type ExternalUserData struct {
	UserID uint   `gorm:"primaryKey;not null"`
	PubKey string `gorm:"not null"`
	User   User   `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// Holds message data
type Message struct {
	MessageID       uint `gorm:"primaryKey;autoincrement;not null"`
	SourceID        uint
	DestinationID   uint
	Stamp           time.Time
	Text            string
	SourceUser      User `gorm:"foreignKey:SourceID;references:UserID;OnDelete:RESTRICT"`
	DestinationUser User `gorm:"foreignKey:DestinationID;references:UserID;OnDelete:RESTRICT"`
}

// Server indentifier that allows a multi-server platform
type Server struct {
	Address  string `gorm:"primaryKey;autoIncrement:false;not null"`
	Port     uint16 `gorm:"primaryKey;autoIncrement:false;not null"`
	ServerID uint   `gorm:"autoIncrement:false;not null"`
}

// Returns the highest ID in the specified table
// This is used to simulate autincremental behaviour in row creation
func getMaxID(db *gorm.DB, table string) uint {
	var maxID uint
	// If the result of the query is null (the table has no rows) a 0 is returned
	row := db.Raw("SELECT IFNULL(MAX(" + tableToID[table] + "), 0) FROM " + table)
	row.Scan(&maxID)
	return maxID
}

// Adds a socket pair to the database if the socket is not on it already. Then,
// returns it
func SaveServer(db *gorm.DB, address string, port uint16) Server {
	// Adds the server to the database only if it is not in it already
	server := Server{ServerID: getMaxID(db, "servers") + 1, Address: address, Port: port}
	if !ServerExists(db, address, port) {
		db.Create(&server)
	} else {
		server.ServerID = GetServer(db, address, port).ServerID
	}
	return server
}

// Returns the server that with the specified socket
func GetServer(db *gorm.DB, address string, port uint16) Server {
	var server Server
	db.Where("address = ? AND port = ?", address, port).First(&server)
	return server
}

// Returns true if the specified socket exists in the database
func ServerExists(db *gorm.DB, address string, port uint16) bool {
	var found bool = false
	db.Raw("SELECT EXISTS(SELECT * FROM servers WHERE address = ? AND port = ?) AS found", address, port).Scan(&found)
	return found
}

// Returns true if the specified username and server defines a local user in the database
func LocalUserExists(db *gorm.DB, username string) bool {
	var found bool = false
	db.Raw("SELECT EXISTS(SELECT * FROM users, local_user_data WHERE users.user_id = local_user_data.user_id AND username = ?) AS found", username).Scan(&found)
	return found
}

// Returns the user that is defined by the username and server
func getUser(db *gorm.DB, username string) User {
	user := User{Username: username}
	db.First(&user)
	return user
}

// Returns the local user that is defined by the specified username and server
func GetLocalUser(db *gorm.DB, username string) LocalUserData {
	localUser := LocalUserData{User: User{Username: username}, UserID: getUser(db, username).UserID}
	db.First(&localUser)
	return localUser
}

// Gets all the local usernames (used in USRS to print local usernames)
func GetAllLocalUsernames(db *gorm.DB) []string {
	var usernames []string
	db.Raw("SELECT username FROM users, local_user_data WHERE local_user_data.user_id = users.user_id").Scan(&usernames)
	return usernames

}

// Adds a user autoincrementally in the database and then returns it
func addUser(db *gorm.DB, username string, serverID uint) (User, error) {
	user := User{UserID: getMaxID(db, "users") + 1, Username: username, ServerID: serverID}
	result := db.Create(&user)
	if result.RowsAffected != 1 {
		return User{}, fmt.Errorf("unexpected number of rows affected in User creation")
	}
	return user, nil
}

// Adds a local user autoincrementally in the database and then returns it
func AddLocalUser(db *gorm.DB, username string, hashPass string, prvKeyPEM string, serverID uint) error {
	// Attempts to create the user. If there's a user with that username and server already
	// the local user will not be created
	user, userErr := addUser(db, username, serverID)
	if userErr != nil {
		return userErr
	}
	localUser := LocalUserData{User: user, UserID: user.UserID, Password: hashPass, PrvKey: prvKeyPEM}

	result := db.Create(&localUser)
	if result.RowsAffected != 1 {
		return fmt.Errorf("unexpected number of rows affected in LocalUserData creation")
	}
	return nil
}

// Adds a local user autoincrementally in the database and then returns it
func AddExternalUser(db *gorm.DB, username string, pubKeyPEM string, serverID uint) error {
	// Attempts to create the user. If there's a user with that username and server already
	// the local user will not be created
	user, userErr := addUser(db, username, serverID)
	if userErr != nil {
		return userErr
	}
	externalUser := ExternalUserData{User: user, UserID: user.UserID, PubKey: pubKeyPEM}
	result := db.Create(&externalUser)
	if result.RowsAffected != 1 {
		return fmt.Errorf("unexpected number of rows affected in ExternalUserData creation")
	}
	return nil
}

// Returns the external user that is defined by the specified username and server
func GetExternalUser(db *gorm.DB, username string) ExternalUserData {
	externalUser := ExternalUserData{User: User{Username: username}, UserID: getUser(db, username).UserID}
	db.First(&externalUser)
	return externalUser
}

// Returns true if the specified username and server defines an external user in the database
func ExternalUserExists(db *gorm.DB, username string) bool {
	var found bool = false
	db.Raw("SELECT EXISTS(SELECT * FROM users, external_user_data WHERE users.user_id = external_user_data.user_id AND username = ?) AS found", username).Scan(&found)
	return found
}

// Creates a message
func StoreMessage(db *gorm.DB, src string, dst string, text string, stamp time.Time) error {
	msg := Message{SourceID: getUser(db, src).UserID, DestinationID: getUser(db, dst).UserID, Text: text, Stamp: stamp}
	result := db.Create(&msg)

	if result.RowsAffected != 1 {
		return fmt.Errorf("unexpected number of rows affected in Message creation")
	}
	return nil
}
