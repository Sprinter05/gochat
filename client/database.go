package main

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Map that contains the ID column of each non-autoincremental table
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
	UserID   uint   `gorm:"primaryKey;not null"`
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

func getMaxID(db *gorm.DB, table string) uint {
	var maxID uint
	// If the result of the query is null (the table has no rows) a 0 is returned
	row := db.Raw("SELECT IFNULL(MAX(" + tableToID[table] + "), 0) FROM " + table)
	row.Scan(&maxID)
	return maxID
}

// Adds a socket pair to the database if the socket is not on it already. Then,
// it returns it
func SaveServer(db *gorm.DB, address string, port uint16) Server {
	// Adds the server to the database only if it is not in it already
	server := Server{ServerID: getMaxID(db, "servers") + 1, Address: address, Port: port}
	if !serverExists(db, address, port) {
		db.Create(&server)
	} else {
		server.ServerID = getServer(db, address, port).ServerID
	}
	return server
}

func getServer(db *gorm.DB, address string, port uint16) Server {
	var server Server
	db.Where("address = ? AND port = ?", address, port).First(&server)
	return server
}

func serverExists(db *gorm.DB, address string, port uint16) bool {
	var found bool = false
	db.Raw("SELECT EXISTS(SELECT * FROM servers WHERE address = ? AND port = ?) AS found", address, port).Scan(&found)
	return found
}

func LocalUserExists(db *gorm.DB, username string) bool {
	var found bool = false
	db.Raw("SELECT EXISTS(SELECT * FROM users, local_user_data WHERE users.user_id = local_user_data.user_id AND username = ?) AS found", username).Scan(&found)
	return found
}

func GetLocalUser(db *gorm.DB, username string) LocalUserData {
	localUser := LocalUserData{User: User{Username: username}}
	db.First(&localUser)
	return localUser
}

func AddLocalUser(db *gorm.DB, username string, hashPass string, prvKeyPEM string, data ShellData) error {
	user := User{UserID: getMaxID(db, "users"), Username: username, ServerID: data.Server.ServerID}
	localUser := LocalUserData{User: user, Password: hashPass, PrvKey: prvKeyPEM}

	result := db.Create(&localUser)
	if result.RowsAffected != 1 {
		return fmt.Errorf("unexpected number of rows affected")
	}
	return nil
}
