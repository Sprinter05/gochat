// This package is used to allow the server to abstract
// the communication between itself and the database.
//
// It is important to know that it is only designed to work
// with a MariaDB database.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	stdlog "log"
	"os"
	"time"

	"github.com/Sprinter05/gochat/internal/log"

	driver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/* UTILITIES */

// Gets the necessary environment variables
// to connect to the database.
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

// Connects to the database using an optional logger
// that can be passed as a parameter
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

	// Open the connection
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

	// Check if the database can be pinged
	sqldb, _ := db.DB()
	if e := sqldb.Ping(); e != nil {
		log.Fatal("database ping", e)
	}

	// Run migrations
	Migrate(db)

	return db
}

/* TYPES */

// Specifies the permissions this database
// implementation stores.
type Permission int8

const (
	USER  Permission = iota // Lowest level
	ADMIN                   // Can perform admin operations
	OWNER                   // Can designate new administrators
)

var permsToString map[Permission]string = map[Permission]string{
	USER:  "USER",
	ADMIN: "ADMIN",
	OWNER: "OWNER",
}

var stringToPerms map[string]Permission = map[string]Permission{
	"USER":  USER,
	"ADMIN": ADMIN,
	"OWNER": OWNER,
}

/* MODELS */

// Identifies users stored in the database
type User struct {
	UserID     uint           `gorm:"primaryKey;autoIncrement;not null"`
	Username   string         `gorm:"unique;not null;size:32"`
	Pubkey     sql.NullString `gorm:"unique;size:2047"`
	Permission Permission     `gorm:"not null;default:0"`
}

// Identifies messages stored in the database
type Message struct {
	SrcUser     uint      `gorm:"not null;check:src_user <> dst_user"`
	DstUser     uint      `gorm:"not null"`
	Message     string    `gorm:"not null;size:2047"`
	Stamp       time.Time `gorm:"not null;default:CURRENT_TIMESTAMP()"`
	Source      User      `gorm:"foreignKey:src_user;OnDelete:RESTRICT"`
	Destination User      `gorm:"foreignKey:dst_user;OnDelete:RESTRICT"`
}

/* ERRORS */

var (
	ErrorNotFound      = errors.New("record not found in the database")                // record not found in the database
	ErrorDuplicatedKey = errors.New("unique key constraint violated due to duplicate") // unique key constraint violated due to duplicate
	ErrorForeignKey    = errors.New("foreign key restrict violation")                  // foreign key restrict violation
	ErrorConsistency   = errors.New("invalid data found in the database")              // invalid data found in the database
	ErrorEmpty         = errors.New("empty result found")                              // empty result found
)

/* FUNCTIONS */

// Returns the name string asocciated to a
// permission level or an empty string if the
// permission level was not found.
func PermissionString(p Permission) string {
	v, ok := permsToString[p]
	if !ok {
		return ""
	}
	return v
}

// Returns the permission level asocciated to
// a string or -1 if the permission level
// was not found.
func StringPermission(s string) Permission {
	v, ok := stringToPerms[s]
	if !ok {
		return -1
	}
	return v
}

// Runs database migrations, ensuring all tables
// are up to date.
func Migrate(db *gorm.DB) {
	err := db.Set(
		"gorm:table_options",
		"ENGINE=InnoDB",
	).AutoMigrate(&User{}, &Message{})
	if err != nil {
		log.Fatal("database migrations", err)
	}
}
