package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/model"

	driver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/* UTILITIES */

// Gets the necessary environment variables
func getDBEnv() string {
	user, ok := os.LookupEnv("DB_USER")
	if !ok {
		log.Environ("DB_USER")
	}

	pswd, ok := os.LookupEnv("DB_PSWD")
	if !ok {
		log.Environ("DB_PSWD")
	}

	addr, ok := os.LookupEnv("DB_ADDR")
	if !ok {
		log.Environ("DB_ADDR")
	}

	port, ok := os.LookupEnv("DB_PORT")
	if !ok {
		log.Environ("DB_PORT")
	}

	name, ok := os.LookupEnv("DB_NAME")
	if !ok {
		log.Environ("DB_NAME")
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
func Connect(logfile *stdlog.Logger) *gorm.DB {
	access := getDBEnv()

	var dblog logger.Interface = nil
	if logfile != nil {
		dblog = logger.New(
			logfile,
			logger.Config{
				LogLevel:             logger.Info,
				ParameterizedQueries: false,
			},
		)
	}

	db, err := gorm.Open(
		driver.Open(access),
		&gorm.Config{
			PrepareStmt:    true,
			TranslateError: true,
			Logger:         dblog,
		},
	)
	if err != nil {
		log.Fatal("database login", err)
	}

	// Run migrations
	Migrate(db)

	return db
}

/* MODELS */

// Identifies the model of a user in the database
type User struct {
	UserID     uint             `gorm:"primaryKey;autoIncrement;not null"`
	Username   model.Username   `gorm:"unique;not null;size:32"`
	Pubkey     sql.NullString   `gorm:"unique;size:2047"`
	Permission model.Permission `gorm:"not null;default:0"`
}

// Identifies the model of a message in the database
type Message struct {
	SrcUser     uint      `gorm:"not null;check:src_user <> dst_user"`
	DstUser     uint      `gorm:"not null"`
	Message     string    `gorm:"not null;size:2047"`
	Stamp       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP()"`
	Source      User      `gorm:"foreignKey:src_user;OnDelete:RESTRICT"`
	Destination User      `gorm:"foreignKey:dst_user;OnDelete:RESTRICT"`
}

// Runs migrations for the database
func Migrate(db *gorm.DB) {
	err := db.Set(
		"gorm:table_options",
		"ENGINE=InnoDB",
	).AutoMigrate(&User{}, &Message{})
	if err != nil {
		log.Fatal("database migrations", err)
	}
}

/* QUERIES */

// Returns a user by their username
// This returns a user according to the db model
func QueryUser(db *gorm.DB, uname model.Username) (*User, error) {
	var user User
	res := db.First(&user, "username = ?", uname)
	if res.Error != nil {
		log.DBError(res.Error)
		// No user with that username exists
		if res.Error == gorm.ErrRecordNotFound {
			return nil, model.ErrorDoesNotExist
		}

		return nil, res.Error
	}

	return &user, nil
}

// Gets all messages from the user
func QueryMessages(db *gorm.DB, uname model.Username) ([]model.Message, error) {
	user, err := QueryUser(db, uname)
	if err != nil {
		return nil, err
	}

	// We give it a context so its safe to reuse
	res := db.Model(&Message{}).Select(
		"username", "message", "stamp",
	).Joins(
		"JOIN gc_users u ON gc_messages.src_user = u.user_id",
	).Where(
		"gc_messages.dest_user = ?", user.UserID,
	).WithContext(context.Background())

	var size int64
	res.Count(&size)

	// No results
	if size == 0 {
		return nil, spec.ErrorEmpty
	}

	rows, err := res.Rows()
	if err != nil {
		log.DBError(err)
		return nil, err
	}
	defer rows.Close()

	// We create a preallocated array
	message := make([]model.Message, size)

	for i := 0; rows.Next(); i++ {
		var undec string
		var temp model.Message

		err := rows.Scan(
			&temp.Sender,
			&undec,
			&temp.Stamp,
		)

		if err != nil {
			return nil, err
		}

		// Conversion from hex string
		dec, e := hex.DecodeString(undec)
		if e != nil {
			log.DBFatal("encripted hex message", string(uname), e)
		}
		temp.Content = dec

		message = append(message, temp)
	}

	return message, nil
}

// Lists all usernames in the database
func QueryUsernames(db *gorm.DB) (string, error) {
	var users strings.Builder
	var dbusers []User

	res := db.Select("username").Find(&dbusers)
	if res.Error != nil {
		log.DBError(res.Error)
		return "", res.Error
	}

	if len(dbusers) == 0 {
		return "", spec.ErrorEmpty
	}

	for _, v := range dbusers {
		// Append to buffer
		users.WriteString(string(v.Username) + "\n")
	}

	// Return result without the last newline
	l := users.Len()
	slice := users.String()
	return slice[:l-1], nil
}

/* INSERTIONS */

// Inserts a user into a database, key must be in PEM format
func InsertUser(db *gorm.DB, uname model.Username, pubkey []byte) error {
	// Public key must be a sql null string
	res := db.Create(&User{
		Username: uname,
		Pubkey: sql.NullString{
			String: string(pubkey),
			Valid:  true,
		},
	})

	if res.Error != nil {
		log.DBError(res.Error)
		return res.Error
	}

	return nil
}

// Adds a message to the users message cache
func CacheMessage(db *gorm.DB, dst model.Username, msg model.Message) error {
	srcuser, srcerr := QueryUser(db, msg.Sender)
	if srcerr != nil {
		return srcerr
	}

	dstuser, dsterr := QueryUser(db, dst)
	if dsterr != nil {
		return dsterr
	}

	// Encode encrypted array to string
	str := hex.EncodeToString([]byte(msg.Content))
	res := db.Create(&Message{
		SrcUser: srcuser.UserID,
		DstUser: dstuser.UserID,
		Message: str,
		Stamp:   msg.Stamp,
	})

	if res.Error != nil {
		log.DBError(res.Error)
		return res.Error
	}

	return nil
}

/* UPDATES */

// Prevents a user from logging in
func RemoveKey(db *gorm.DB, uname model.Username) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	// Set public key to null
	user.Pubkey = sql.NullString{
		Valid: false,
	}

	res := db.Save(&user)
	if res.Error != nil {
		log.DBError(res.Error)
		return res.Error
	}

	return nil
}

// Changes the permissions of a user
func ChangePermission(db *gorm.DB, uname model.Username, perm model.Permission) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	user.Permission = perm

	res := db.Save(&user)
	if res.Error != nil {
		log.DBError(res.Error)
		return res.Error
	}

	return nil
}

/* DELETIONS */

// Removes a user from the database
// Fails if the user has messages pending
func RemoveUser(db *gorm.DB, uname model.Username) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	res := db.Delete(&user)
	if res.Error != nil {
		log.DBError(res.Error)
		// Check if the error is the foreign key constraint
		if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
			return model.ErrorDBConstraint
		}
		return res.Error
	}

	return nil
}

// Removes all cached messages from a user before a given stamp
// This is done to prevent messages from being lost due to concurrency
func RemoveMessages(db *gorm.DB, uname model.Username, stamp time.Time) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	// Delete, checking the timestamp
	res := db.Delete(
		&Message{},
		"dst_user = ? AND stamp <= ?",
		user.UserID,
		stamp,
	)

	if res.Error != nil {
		log.DBError(res.Error)
		return res.Error
	}

	return nil
}
