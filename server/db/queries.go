package db

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"

	"gorm.io/gorm"
)

/* QUERIES */

// Returns a user by their username
// This returns a user according to the db model
func QueryUser(db *gorm.DB, uname string) (*User, error) {
	var user User
	res := db.First(&user, "username = ?", uname)
	if res.Error != nil {
		log.DBError(res.Error)
		// No user with that username exists
		if res.Error == gorm.ErrRecordNotFound {
			return nil, ErrorNotFound
		}

		return nil, res.Error
	}

	return &user, nil
}

// Gets all messages from the user
func QueryMessages(db *gorm.DB, uname string) ([]*spec.Message, error) {
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
	pre := res.Count(&size)
	if pre.Error != nil {
		log.DBError(pre.Error)
		return nil, pre.Error
	}

	// No results
	if size == 0 {
		return nil, ErrorEmpty
	}

	rows, err := res.Rows()
	if err != nil {
		log.DBError(err)
		return nil, err
	}
	defer rows.Close()

	// We create a preallocated array
	messages := make([]*spec.Message, 0, size)

	for i := 0; rows.Next(); i++ {
		var undec string
		var temp spec.Message

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
			log.DBFatal("encripted hex message", uname, e)
		}
		temp.Content = dec

		messages = append(messages, &temp)
	}

	return messages, nil
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
		return "", ErrorEmpty
	}

	// Preallocate strings builder
	for _, v := range dbusers {
		users.Grow(len(v.Username))
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
func InsertUser(db *gorm.DB, uname string, pubkey []byte) error {
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
		// Content already exists
		if res.Error == gorm.ErrDuplicatedKey {
			return ErrorDuplicatedKey
		}
		return res.Error
	}

	return nil
}

// Adds a message to the users message cache
func CacheMessage(db *gorm.DB, dst string, msg spec.Message) error {
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
func RemoveKey(db *gorm.DB, uname string) error {
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
func ChangePermission(db *gorm.DB, uname string, perm Permission) error {
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
func RemoveUser(db *gorm.DB, uname string) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	res := db.Delete(&user)
	if res.Error != nil {
		log.DBError(res.Error)
		// Check if the error is the foreign key constraint
		if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
			return ErrorForeignKey
		}
		return res.Error
	}

	return nil
}

// Removes all cached messages from a user before a given stamp
// This is done to prevent messages from being lost due to concurrency
func RemoveMessages(db *gorm.DB, uname string, stamp time.Time) error {
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
