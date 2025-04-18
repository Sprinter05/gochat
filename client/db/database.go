package main

import (
	"time"
)

// Generic user table that defines the columns every user shares
type User struct {
	ID       uint   `gorm:"primaryKey;autoIncrement;not null"`
	Username string `gorm:"unique;not null"`
	ServerID uint   `gorm:"primaryKey;not null"`
	Server   Server `gorm:"foreignKey:ServerID;OnDelete:CASCADE"`
}

// User extension dedicated to shell-created users.
// The passwords should be hashed and the private
// keys need to be stored
type LocalUserData struct {
	ID       uint   `gorm:"primaryKey;not null"`
	Password string `gorm:"not null"`
	PrvKey   string
	User     User `gorm:"foreignKey:ID;OnDelete:CASCADE"`
}

// User extension dedicated to REQ'd users. Only
// their public key is needed to encrypt messages
// to them
type ExternalUserData struct {
	ID     uint   `gorm:"primaryKey;not null"`
	PubKey string `gorm:"not null"`
	User   User   `gorm:"foreignKey:ID;OnDelete:CASCADE"`
}

// Holds message data
type Message struct {
	SourceID        uint `gorm:"primaryKey;not null"`
	DestinationID   uint
	Stamp           time.Time `gorm:"primaryKey;not null"`
	Text            string
	SourceUser      User `gorm:"foreignKey:SourceID;OnDelete:RESTRICT"`
	DestinationUser User `gorm:"foreignKey:DestinationID;OnDelete:RESTRICT"`
}

// Server indentifier that allows a multi-server platform
type Server struct {
	ID      uint `gorm:"primaryKey;autoIncrement;not null"`
	Port    uint `gorm:"primaryKey;not null"`
	Address string
}
