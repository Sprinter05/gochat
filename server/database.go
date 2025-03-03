package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"

	driver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	logger "gorm.io/gorm/logger"
)

/* UTILITIES */

// Gets the environment variables necessary
func getDBEnv() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		gclog.Environ("DB_USER")
	}

	pswd, ok := os.LookupEnv("DB_PSWD")
	if !ok {
		gclog.Environ("DB_PSWD")
	}

	addr, ok := os.LookupEnv("DB_ADDR")
	if !ok {
		gclog.Environ("DB_ADDR")
	}

	port, ok := os.LookupEnv("DB_PORT")
	if !ok {
		gclog.Environ("DB_PORT")
	}

	name, ok := os.LookupEnv("DB_NAME")
	if !ok {
		gclog.Environ("DB_NAME")
	}

	// Get formatted string
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=True",
		user,
		pswd,
		addr,
		port,
		name,
	)
}

// Connects to the database using the environment file
func connectDB(logfile *log.Logger) *gorm.DB {
	access := getDBEnv()
	dblog := logger.New(
		logfile,
		logger.Config{
			LogLevel:             logger.Info,
			ParameterizedQueries: false,
		},
	)
	db, err := gorm.Open(
		driver.Open(access),
		&gorm.Config{
			PrepareStmt:    true,
			TranslateError: true,
			Logger:         dblog,
		},
	)
	if err != nil {
		gclog.Fatal("database login", err)
	}

	// Run migrations
	migrate(db)

	return db
}

/* MODELS */

type gcUser struct {
	UserID     uint           `gorm:"primaryKey;autoIncrement;not null"`
	Username   string         `gorm:"unique;not null;size:32"`
	Pubkey     sql.NullString `gorm:"unique;size:2047"`
	Permission Permission     `gorm:"not null;default:0"`
}

type gcMessage struct {
	SrcUser     uint      `gorm:"not null;check:src_user <> dst_user"`
	DstUser     uint      `gorm:"not null"`
	Message     string    `gorm:"not null;size:2047"`
	Stamp       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP()"`
	Source      gcUser    `gorm:"foreignKey:src_user;OnDelete:RESTRICT"`
	Destination gcUser    `gorm:"foreignKey:dst_user;OnDelete:RESTRICT"`
}

func migrate(db *gorm.DB) {
	err := db.Set(
		"gorm:table_options",
		"ENGINE=InnoDB",
	).AutoMigrate(&gcUser{}, &gcMessage{})
	if err != nil {
		gclog.Fatal("database migrations", err)
	}
}

/* QUERIES */

// Returns a user by their username
// This returns a user according to the db model
func queryDBUser(db *gorm.DB, uname username) (*gcUser, error) {
	var user gcUser
	res := db.First(&user, "username = ?", uname)
	if res.Error != nil {
		gclog.DBError(res.Error)
		// No user with that username exists
		if res.Error == gorm.ErrRecordNotFound {
			return nil, ErrorDoesNotExist
		}

		return nil, res.Error
	}

	return &user, nil
}

// Returns a user struct by their username
func queryUser(db *gorm.DB, uname username) (*User, error) {
	dbuser, err := queryDBUser(db, uname)
	if err != nil {
		return nil, err
	}

	// Check that the permissions are correct
	if dbuser.Permission > OWNER {
		return nil, ErrorInvalidValue
	}

	// Check that the public key is not null
	if !dbuser.Pubkey.Valid {
		return nil, ErrorDeregistered
	}

	// Turn it into a public key from PEM certificate
	key, err := gc.PEMToPubkey([]byte(dbuser.Pubkey.String))
	if err != nil {
		return nil, err
	}

	// Connection should be assigned by the calling function
	// Only if necessary
	return &User{
		conn:   nil,
		name:   uname,
		pubkey: key,
		perms:  dbuser.Permission,
	}, nil
}

// Gets all messages from the user
// It is expected for the size to be queried previously
func queryMessages(db *gorm.DB, uname username) (*[]Message, error) {
	user, err := queryDBUser(db, uname)
	if err != nil {
		return nil, err
	}

	// We give it a context so its safe to reuse
	res := db.Model(&gcMessage{}).Select(
		"username", "message", "stamp",
	).Joins(
		"JOIN gc_users u ON gc_messages.src_user = u.user_id",
	).Where(
		"gc_messages.dest_user = ?", user.UserID,
	).WithContext(context.Background())

	rows, err := res.Rows()
	if err != nil {
		gclog.DBError(err)
		return nil, err
	}
	defer rows.Close()

	// We create a preallocated array
	var size int64
	res.Count(&size)
	message := make([]Message, size)

	// No results
	if size == 0 {
		return nil, gc.ErrorEmpty
	}

	for i := 0; rows.Next(); i++ {
		var undec string
		var temp Message

		err := rows.Scan(
			&temp.sender,
			&undec,
			&temp.stamp,
		)

		if err != nil {
			return nil, err
		}

		// Conversion from hex string
		dec, _ := hex.DecodeString(undec)
		temp.message = dec

		message = append(message, temp)
	}

	return &message, nil
}

// Lists all usernames in the database
func queryUsernames(db *gorm.DB) (string, error) {
	var users strings.Builder
	var dbusers []gcUser

	res := db.Select("username").Find(&dbusers)
	if res.Error != nil {
		gclog.DBError(res.Error)
		return "", res.Error
	}

	if len(dbusers) == 0 {
		return "", gc.ErrorEmpty
	}

	for _, v := range dbusers {
		// Append to buffer
		users.WriteString(v.Username + "\n")
	}

	// Return result without the last newline
	l := users.Len()
	slice := users.String()
	return slice[:l-1], nil
}

/* INSERTIONS */

// Inserts a user into a database, key must be in PEM format
func insertUser(db *gorm.DB, uname username, pubkey []byte) error {
	// Public key must be a sql null string
	res := db.Create(&gcUser{
		Username: string(uname),
		Pubkey: sql.NullString{
			String: string(pubkey),
			Valid:  true,
		},
	})

	if res.Error != nil {
		gclog.DBError(res.Error)
		return res.Error
	}

	return nil
}

// Adds a message to the users message cache
// The message must be in byte array format since its encrypted
func cacheMessage(db *gorm.DB, dst username, msg Message) error {
	srcuser, srcerr := queryDBUser(db, msg.sender)
	if srcerr != nil {
		return srcerr
	}

	dstuser, dsterr := queryDBUser(db, dst)
	if dsterr != nil {
		return dsterr
	}

	// Encode encrypted array to string
	str := hex.EncodeToString([]byte(msg.message))
	res := db.Create(&gcMessage{
		SrcUser: srcuser.UserID,
		DstUser: dstuser.UserID,
		Message: str,
		Stamp:   msg.stamp,
	})

	if res.Error != nil {
		gclog.DBError(res.Error)
		return res.Error
	}

	return nil
}

/* UPDATES */

// Prevents a user from logging in
func removeKey(db *gorm.DB, uname username) error {
	user, err := queryDBUser(db, uname)
	if err != nil {
		return err
	}

	// Set public key to null
	user.Pubkey = sql.NullString{
		Valid: false,
	}

	res := db.Save(&user)
	if res.Error != nil {
		gclog.DBError(res.Error)
		return res.Error
	}

	return nil
}

// Changes the permissions of a user
func changePermissions(db *gorm.DB, uname username, perm Permission) error {
	user, err := queryDBUser(db, uname)
	if err != nil {
		return err
	}

	user.Permission = perm

	res := db.Save(&user)
	if res.Error != nil {
		gclog.DBError(res.Error)
		return res.Error
	}

	return nil
}

/* DELETIONS */

// Removes a user from the database
func removeUser(db *gorm.DB, uname username) error {
	user, err := queryDBUser(db, uname)
	if err != nil {
		return err
	}

	res := db.Delete(&user)
	if res.Error != nil {
		gclog.DBError(res.Error)
		// Check if the error is the foreign key constraint
		if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
			return ErrorDBConstraint
		}
		return res.Error
	}

	return nil
}

// Removes all cached messages from a user before a given stamp
// This is done to prevent messages from being lost
func removeMessages(db *gorm.DB, uname username, stamp time.Time) error {
	user, err := queryDBUser(db, uname)
	if err != nil {
		return err
	}

	// Delete checking the timestamp
	res := db.Delete(
		&gcMessage{},
		"dst_user = ? AND stamp <= ?",
		user.UserID,
		stamp,
	)

	if res.Error != nil {
		gclog.DBError(res.Error)
		return res.Error
	}

	return nil
}
