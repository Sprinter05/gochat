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

// Returns a database user according to their username
func QueryUser(db *gorm.DB, uname string) (*User, error) {
	var user User
	res := db.First(&user, "username = ?", uname)
	if res.Error != nil {
		log.DBError(res.Error)
		// Abstract with errors of the db package
		if res.Error == gorm.ErrRecordNotFound {
			return nil, ErrorNotFound
		}

		return nil, res.Error
	}

	return &user, nil
}

// Gets all messages directed to the specified user as an array of pointers,
// this was it is easier to pass it around. It uses the specification
// Message type and not the database one due to how messages are stored,
// which will be returned in an encrypted state.
func QueryMessages(db *gorm.DB, uname string) ([]*spec.Message, error) {
	user, err := QueryUser(db, uname)
	if err != nil {
		return nil, err
	}

	// We give it a context so its safe to reuse
	// for first counting and then returning results
	res := db.Model(&Message{}).Select(
		"username", "message", "stamp",
	).Joins(
		"JOIN users u ON messages.src_user = u.user_id",
	).Where(
		"messages.dest_user = ?", user.UserID,
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

	// Preallocate space
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

// Returns a list of all users registered in the database
// as a single string separated by '\n', or an error if
// no users are registered.
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
		users.WriteString(string(v.Username) + "\n")
	}

	// Return result without the last newline
	l := users.Len()
	slice := users.String()
	return slice[:l-1], nil
}

/* INSERTIONS */

// Inserts a user into a database, the public key provided must be
// in the PEM format that the specification uses to prevent future
// errors on retrieval.
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
		// Abstract gorm database error
		if res.Error == gorm.ErrDuplicatedKey {
			return ErrorDuplicatedKey
		}
		return res.Error
	}

	return nil
}

// Cache a message into the database for future retrieval
// by the destination user. Message should be encrypted when
// inserting, as the database makes no checks whatsoever.
func CacheMessage(db *gorm.DB, dst string, msg spec.Message) error {
	srcuser, srcerr := QueryUser(db, msg.Sender)
	if srcerr != nil {
		return srcerr
	}

	dstuser, dsterr := QueryUser(db, dst)
	if dsterr != nil {
		return dsterr
	}

	// Encode encrypted array to string for
	// better compatibility
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

// Prevents a user from logging in by nullifying their public
// key, this is better than deletion in case some messages
// related to this user are still cached.
func RemoveKey(db *gorm.DB, uname string) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

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

// Changes the permission level of a user, according to the ones
// provided in the Permission type.
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

// Attempts to remove a user from the database,
// fails if the user has messages pending, in which
// case it is recommended to use the RemoveKey() function.
func RemoveUser(db *gorm.DB, uname string) error {
	user, err := QueryUser(db, uname)
	if err != nil {
		return err
	}

	res := db.Delete(&user)
	if res.Error != nil {
		log.DBError(res.Error)
		// Abstract gorm error to the caller
		if errors.Is(res.Error, gorm.ErrForeignKeyViolated) {
			return ErrorForeignKey
		}
		return res.Error
	}

	return nil
}

// Removes all cached messages destinated to a given user before a
// given stamp, this is done to prevent messages from being lost
// due to concurrent access. It is advised to use the timestamp
// of the last retrieved message, as that should be the newest one.
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
