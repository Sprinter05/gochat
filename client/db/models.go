package db

// Contains the GORM models needed to interact with the client database

import "time"

// Generic user table that defines the columns every user shares.
type User struct {
	UserID   uint   `gorm:"autoIncrement:false;not null"`
	ServerID uint   `gorm:"primaryKey;autoIncrement:false;not null"`
	Username string `gorm:"primaryKey;not null"`

	Server Server `gorm:"foreignKey:ServerID;references:ServerID;constraint:OnDelete:CASCADE"`
}

// User extension dedicated to locally created users.
// The passwords should be hashed and the private
// keys need to be stored in PEM format.
type LocalUser struct {
	UserID   uint   `gorm:"primaryKey;not null;autoIncrement:false"`
	Password string `gorm:"not null"`
	PrvKey   string

	User User `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// User extension dedicated to REQ'd users. Only
// their public key is needed to encrypt messages
// to them.
type ExternalUser struct {
	UserID uint   `gorm:"primaryKey;not null"`
	PubKey string `gorm:"not null"`

	User User `gorm:"foreignKey:UserID;OnDelete:CASCADE"`
}

// Holds message data.
type Message struct {
	MessageID     uint `gorm:"primaryKey;autoincrement;not null"`
	SourceID      uint
	DestinationID uint
	Stamp         time.Time
	Text          string

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
