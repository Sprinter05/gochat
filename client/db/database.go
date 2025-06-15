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

/* TABLES */

// Map that contains the ID column of each non-autoincremental table, used in getMaxID.
var tableToID = map[string]string{
	"servers": "server_id",
	"users":   "user_id",
}

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
	TLS      bool   `gorm:"not null"`
	ServerID uint   `gorm:"autoIncrement:false;not null"`
	Name     string `gorm:"unique;not null"`
}

/* CONNECTION */

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

/* SERVER QUERIES */

// Adds a socket pair to the database if the socket is not on it already. Then,
// returns it.
func AddServer(db *gorm.DB, address string, port uint16, name string, tls bool) (Server, error) {
	// Adds the server to the database only if it is not in it already
	server := Server{
		ServerID: getMaxID(db, "servers") + 1,
		Address:  address,
		Port:     port,
		Name:     name,
		TLS:      tls,
	}

	svExists, existsErr := ServerExists(db, address, port)
	if existsErr != nil {
		return Server{}, existsErr
	}

	if !svExists {
		result := db.Create(&server)
		if result.Error != nil {
			return server, result.Error
		}
	} else {
		newServer, getErr := GetServer(db, address, port)
		if getErr != nil {
			return Server{}, getErr
		}
		server.ServerID = newServer.ServerID
		server.Name = name
		server.TLS = tls
		result := db.Save(&server)
		if result.Error != nil {
			return Server{}, result.Error
		}
	}

	return server, nil
}

// Returns the server with the specified socket.
func GetServer(db *gorm.DB, address string, port uint16) (Server, error) {
	var server Server
	result := db.Raw(
		`SELECT * FROM servers
		WHERE address = ? AND port = ?`,
		address, port,
	).Scan(&server)

	return server, result.Error
}

// Returns the server with the specified name.
func GetServerByName(db *gorm.DB, name string) (Server, error) {
	var server Server
	result := db.Where("name = ?", name).First(&server)
	return server, result.Error
}

// Deletes a server from the database.
func RemoveServer(db *gorm.DB, address string, port uint16) error {
	sv, err := GetServer(db, address, port)
	if err != nil {
		return err
	}

	result := db.Delete(&sv)
	return result.Error
}

// Returns all servers
func GetAllServers(db *gorm.DB) ([]Server, error) {
	var servers []Server
	result := db.Raw(
		`SELECT * FROM servers`,
	).Scan(&servers)
	return servers, result.Error
}

// Returns true if the specified socket exists in the database.
func ServerExists(db *gorm.DB, address string, port uint16) (bool, error) {
	var found bool
	result := db.Raw(
		`SELECT EXISTS(
			SELECT * FROM servers 
			WHERE address = ? AND port = ?
		) AS found`,
		address, port,
	).Scan(&found)

	return found, result.Error
}

// Updates TLS data about a server
func ChangeServerTLS(db *gorm.DB, address string, port uint16, tls bool) error {
	sv, err := GetServer(db, address, port)
	if err != nil {
		return err
	}

	sv.TLS = tls
	result := db.Save(&sv)
	return result.Error
}

/* USER QUERIES */

// Returns the user that is defined by the username and server.
func GetUser(db *gorm.DB, username string, address string, port uint16) (User, error) {
	sv, err := GetServer(db, address, port)
	if err != nil {
		return User{}, err
	}

	user := User{Username: username, ServerID: sv.ServerID}
	result := db.First(&user)
	return user, result.Error
}

// // Returns the user by a given ID (only used for specific cases)
// func GetUserByID(db *gorm.DB, userID uint, serverID uint) (User, error) {
// 	if userID == 0 || serverID == 0 {
// 		return User{}, ErrorUnexpectedzero
// 	}

// 	user := User{}
// 	result := db.Where("user_id = ? AND server_id = ?", userID, serverID).First(&user)
// 	return user, result.Error
// }

// Returns the local user that is defined by the specified username and server.+
func GetLocalUser(db *gorm.DB, username string, address string, port uint16) (LocalUser, error) {
	user, err := GetUser(db, username, address, port)
	if err != nil {
		return LocalUser{}, err
	}

	localUser := LocalUser{UserID: user.UserID}
	result := db.First(&localUser)
	localUser.User = user
	return localUser, result.Error
}

// Gets all the local usernames from a specific server (used in USRS to print local usernames).
// Fils the User foreign key
func GetServerLocalUsers(db *gorm.DB, address string, port uint16) ([]LocalUser, error) {
	var users []LocalUser

	sv, err := GetServer(db, address, port)
	if err != nil {
		return nil, err
	}

	result := db.Raw(
		`SELECT *
		FROM local_users lu JOIN users u ON lu.user_id = u.user_id
		WHERE u.server_id = ?`,
		sv.ServerID,
	).Scan(&users)

	for i, v := range users {
		var user User
		db.Where("user_id = ?", v.UserID).Find(&user)

		users[i].User = user
	}

	return users, result.Error
}

// Gets all the external usernames obtained with the REQ command
func GetRequestedUsers(db *gorm.DB) ([]ExternalUser, error) {
	var users []ExternalUser

	result := db.Raw(
		`SELECT *
		FROM external_users`,
	).Scan(&users)

	for i, v := range users {
		var user User
		db.Where("user_id = ?", v.UserID).Find(&user)

		users[i].User = user
	}

	return users, result.Error
}

// Gets all the local usernames from every registered server
// (used in USRS to print local usernames).
// Fills both the user and server foreign key
func GetAllLocalUsers(db *gorm.DB) ([]LocalUser, error) {
	var users []LocalUser

	result := db.Raw(
		`SELECT * FROM local_users`,
	).Scan(&users)

	for i, v := range users {
		var user User
		db.Where("user_id = ?", v.UserID).Find(&user)

		var server Server
		db.Where("server_id = ?", user.ServerID).Find(&server)
		user.Server = server

		users[i].User = user
	}

	return users, result.Error
}

// Returns true if the specified username and server defines a local user in the database.
func LocalUserExists(db *gorm.DB, username string, address string, port uint16) (bool, error) {
	var found bool

	sv, err := GetServer(db, address, port)
	if err != nil {
		return found, err
	}

	result := db.Raw(
		`SELECT EXISTS(
			SELECT *
			FROM users u JOIN local_users lu ON u.user_id = lu.user_id
			WHERE u.username = ? AND u.server_id = ?
		) AS found`,
		username, sv.ServerID,
	).Scan(&found)

	return found, result.Error
}

// Adds a local user autoincrementally in the database and then returns it.
func AddLocalUser(db *gorm.DB, username string, hashPass string, prvKeyPEM string, address string, port uint16) (LocalUser, error) {
	sv, err := GetServer(db, address, port)
	if err != nil {
		return LocalUser{}, err
	}

	user, err := GetUser(db, username, address, port)
	if err != nil {
		// Attempts to create the user. If there's a user with that username and server already
		// the local user will not be created
		new, userErr := addUser(db, username, sv.ServerID)
		if userErr != nil {
			return LocalUser{}, userErr
		}
		user = new
	}

	localUser := LocalUser{
		User:     user,
		UserID:   user.UserID,
		PrvKey:   prvKeyPEM,
		Password: hashPass,
	}

	result := db.Create(&localUser)
	return localUser, result.Error
}

// Adds a local user autoincrementally in the database and then returns it.
func AddExternalUser(db *gorm.DB, username string, pubKeyPEM string, address string, port uint16) (ExternalUser, error) {
	sv, err := GetServer(db, address, port)
	if err != nil {
		return ExternalUser{}, err
	}

	user, err := GetUser(db, username, address, port)
	if err != nil {
		// Attempts to create the user. If there's a user with that username and server already
		// the local user will not be created
		new, userErr := addUser(db, username, sv.ServerID)
		if userErr != nil {
			return ExternalUser{}, userErr
		}
		user = new
	}

	externalUser := ExternalUser{
		User:   user,
		UserID: user.UserID,
		PubKey: pubKeyPEM,
	}

	result := db.Create(&externalUser)
	return externalUser, result.Error
}

// Returns the external user that is defined by the specified username and server.
func GetExternalUser(db *gorm.DB, username string, address string, port uint16) (ExternalUser, error) {
	user, err := GetUser(db, username, address, port)
	if err != nil {
		return ExternalUser{}, err
	}

	externalUser := ExternalUser{UserID: user.UserID}
	externalUser.User = user
	result := db.First(&externalUser)
	return externalUser, result.Error
}

// Returns true if the specified username and server defines an external user in the database.
func ExternalUserExists(db *gorm.DB, username string, address string, port uint16) (bool, error) {
	var found bool

	sv, err := GetServer(db, address, port)
	if err != nil {
		return found, err
	}

	result := db.Raw(
		`SELECT EXISTS(
			SELECT * 
			FROM users u JOIN external_users eu ON u.user_id = eu.user_id
			WHERE u.username = ? AND u.server_id = ?
		) AS found`,
		username, sv.ServerID,
	).Scan(&found)

	return found, result.Error
}

/* MESSAGES */

// Adds a message to the database and returns it.
func StoreMessage(db *gorm.DB, src, dst string, address string, port uint16, text string, stamp time.Time) (Message, error) {
	source, err := GetUser(db, src, address, port)
	if err != nil {
		return Message{}, nil
	}

	destination, err := GetUser(db, dst, address, port)
	if err != nil {
		return Message{}, nil
	}

	ok, err := findMessage(
		db,
		source.UserID,
		destination.UserID,
		stamp,
		text,
	)
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		SourceID:      source.UserID,
		DestinationID: destination.UserID,
		Text:          text,
		Stamp:         stamp,
	}

	if !ok {
		result := db.Create(&msg)
		if result.Error != nil {
			return Message{}, result.Error
		}
	}

	return msg, nil
}

// Returns a slice with every message between two users in a range of time
func GetUsersMessagesLimit(db *gorm.DB, src, dst string, address string, port uint16, limit time.Time) ([]Message, error) {
	var messages []Message

	source, err := GetUser(db, src, address, port)
	if err != nil {
		return nil, err
	}

	destination, err := GetUser(db, dst, address, port)
	if err != nil {
		return nil, err
	}

	result := db.Where(
		`(source_id = ? AND destination_id = ?) 
		OR 
		(source_id = ? AND destination_id = ?)`,
		source.UserID, destination.UserID,
		destination.UserID, source.UserID,
	).Where("stamp < ?", limit).Find(&messages)
	if result.Error != nil {
		return nil, result.Error
	}
	return messages, nil
}

// Returns a slice with all messages between two users
func GetAllUsersMessages(db *gorm.DB, src, dst string, address string, port uint16) ([]Message, error) {
	var messages []Message

	source, err := GetUser(db, src, address, port)
	if err != nil {
		return nil, err
	}

	destination, err := GetUser(db, dst, address, port)
	if err != nil {
		return nil, err
	}

	result := db.Where(
		`(source_id = ? AND destination_id = ?) 
		OR 
		(source_id = ? AND destination_id = ?)`,
		source.UserID, destination.UserID,
		destination.UserID, source.UserID,
	).Order("stamp ASC").Find(&messages)

	for i, v := range messages {
		if v.SourceID == source.UserID {
			messages[i].SourceUser = source
			messages[i].DestinationUser = destination
		} else {
			messages[i].SourceUser = destination
			messages[i].DestinationUser = source
		}
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return messages, nil
}

func DeleteConversation(db *gorm.DB, src, dst string, address string, port uint16) error {
	source, err := GetUser(db, src, address, port)
	if err != nil {
		return err
	}

	destination, err := GetUser(db, dst, address, port)
	if err != nil {
		return err
	}

	result := db.Where(
		`(source_id = ? AND destination_id = ?) 
		OR 
		(source_id = ? AND destination_id = ?)`,
		source.UserID, destination.UserID,
		destination.UserID, source.UserID,
	).Delete(&Message{})

	return result.Error
}
