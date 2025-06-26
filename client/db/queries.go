package db

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

/* SERVER QUERIES */

// Adds a socket pair to the database if the socket
// is not on it already. Also returns said server.
func AddServer(db *gorm.DB, address string, port uint16, name string, tls bool) (Server, error) {
	id := getMaxID(db, "servers") + 1
	server := Server{
		ServerID: id,
		Address:  address,
		Port:     port,
		TLS:      tls,
	}

	// If the name is empty, a default name is set
	if name == "" {
		name = fmt.Sprintf("Default-%d", id)
		server.Name = name
	} else {
		server.Name = name
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

// Returns all servers in the database.
func GetAllServers(db *gorm.DB) ([]Server, error) {
	var servers []Server
	result := db.Raw(
		`SELECT * FROM servers
		ORDER BY server_ID ASC`,
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

// Returns true if the specified named server exists in the database.
func ServerExistsByName(db *gorm.DB, name string) (bool, error) {
	var found bool
	result := db.Raw(
		`SELECT EXISTS(
			SELECT * FROM servers 
			WHERE name = ?
		) AS found`,
		name,
	).Scan(&found)

	return found, result.Error
}

// Update information about a server using its internal ID.
// Values provided as "any" must be a pointer
func UpdateServer(db *gorm.DB, data any, column string, value any) error {
	server, ok := data.(*Server)
	if !ok {
		return ErrorInvalidObject
	}

	result := db.Model(&Server{}).
		Where("server_id = ?", server.ServerID).
		Update(
			column, value,
		)

	return result.Error
}

// Updates TLS data about a server.
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

// Returns the local user that is defined by the specified username and server.
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

// Gets all the local usernames from a specific server.
// Fils the User foreign key.
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

// Gets all the external users found in the database.
func GetRequestedUsers(db *gorm.DB) ([]ExternalUser, error) {
	var users []ExternalUser

	result := db.Raw(
		`SELECT *
		FROM external_users`,
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

// Gets all the local usernames from every registered server.
// Fills both the user and server foreign key.
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

// Returns true if the specified username and server
// defines a local user in the database.
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

// Adds a local user autoincrementally
// in the database and then returns it.
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
		Config:   map[string]any{},
	}

	result := db.Create(&localUser)
	return localUser, result.Error
}

// Adds a local user autoincrementally
// in the database and then returns it.
func DeleteLocalUser(db *gorm.DB, username string, address string, port uint16) error {
	user, err := GetUser(db, username, address, port)
	if err != nil {
		return err
	}

	result := db.Raw(
		`DELETE FROM local_users
		WHERE user_id = ?`,
		user.UserID,
	)
	if result.Error != nil {
		return result.Error
	}

	result = db.Delete(user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// Updates the configuration of a local user. It loads
// the json in the object to the database.
func UpdateUserConfig(db *gorm.DB, user LocalUser) error {
	result := db.Model(&LocalUser{}).
		Where("user_id = ?", user.UserID).
		Update(
			"config",
			user.Config,
		)

	return result.Error
}

// Adds a local user autoincrementally
// in the database and then returns it.
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

// Returns the external user that is defined
// by the specified username and server.
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

// Returns true if the specified username and
// server defines an external user in the database.
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

// Returns a slice with every message between
// two users until a certain point in time.
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

// Returns a slice with all messages between two users, filling foreign keys.
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

// Deletes all messages between two specified users in a same server.
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

/* RECOVERY FUNCTIONS */

// Tries to recover all local users not belonging to any server
// given a username
func RecoverUsers(db *gorm.DB, username string) ([]LocalUser, error) {
	var users []LocalUser
	result := db.Raw(
		`SELECT *
		FROM local_users lu JOIN users u ON lu.user_id = u.user_id
		WHERE u.username = ? AND u.server_id NOT IN (
			SELECT server_id
			FROM servers
		)`,
		username,
	).Scan(&users)

	if result.Error != nil {
		return nil, result.Error
	}

	return users, nil
}

// Tries to recover all messages asocciated to a user, separeted
// by conversations
func RecoverMessages(db *gorm.DB, lu LocalUser) ([][]Message, error) {
	var ids []uint

	// Get all people that it has messages with
	result := db.Raw(
		`SELECT DISTINCT(u.user_id)
			FROM messages m JOIN users u ON m.source_id = u.user_id
				OR m.destination_id = u.user_id
			WHERE m.source_id = ? 
				OR m.destination_id = ?
		EXCEPT SELECT user_id
			FROM users
			WHERE user_id = ?`,
		lu.UserID, lu.UserID, lu.UserID,
	).Scan(&ids)

	if result.Error != nil {
		return nil, result.Error
	}

	// Preallocate array
	convos := make([][]Message, 0, len(ids))

	// Used to fill foreign keys
	user, err := getUserByID(db, lu.UserID)
	if err != nil {
		return nil, err
	}

	// Get all conversations
	for _, v := range ids {
		var messages []Message
		result := db.Raw(
			`SELECT *
			FROM messages
			WHERE (source_id = ? AND destination_id = ?) 
				OR (source_id = ? AND destination_id = ?)
			ORDER BY stamp ASC`,
			lu.UserID, v,
			v, lu.UserID,
		).Scan(&messages)

		if result.Error != nil {
			return nil, result.Error
		}

		dest, err := getUserByID(db, v)
		if err != nil {
			return nil, err
		}

		// Fill foreign keys
		for i, m := range messages {
			if m.SourceID == user.UserID {
				messages[i].SourceUser = user
				messages[i].DestinationUser = dest
			} else {
				messages[i].SourceUser = dest
				messages[i].DestinationUser = user
			}
		}

		convos = append(convos, messages)
	}

	return convos, nil
}

// Cleans a user that has been recovered
func CleanupUser(db *gorm.DB, lu LocalUser) error {
	result := db.Delete(&lu)
	if result.Error != nil {
		return result.Error
	}

	user, err := getUserByID(db, lu.UserID)
	if err != nil {
		return err
	}

	result = db.Delete(&user)
	if result.Error != nil {
		return result.Error
	}

	return nil
}
