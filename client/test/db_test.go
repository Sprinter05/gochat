package test

import (
	"testing"

	"github.com/Sprinter05/gochat/client/db"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Creates a brand new database each time a test is ran
func setup(t *testing.T) *gorm.DB {
	tdb, err := gorm.Open(sqlite.Open("test.db"))
	if err != nil {
		t.Error(err)
	}

	// Deletes all rows of all tables
	tables := []any{&db.Server{}, &db.User{}, &db.LocalUser{}, db.ExternalUser{}, db.Message{}}
	for _, table := range tables {
		tdb.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(table)
	}

	tdb.AutoMigrate(db.Server{}, db.User{}, db.LocalUser{}, db.ExternalUser{}, db.Message{})
	return tdb
}

func TestServer(t *testing.T) {
	testDB := setup(t)
	_, saveErr := db.SaveServer(testDB, "127.0.0.1", 9037, "testserver")
	if saveErr != nil {
		t.Error(saveErr)
	}

	server, getErr := db.GetServer(testDB, "127.0.0.1", 9037)
	if getErr != nil {
		t.Error(getErr)
	}

	expServer := db.Server{ServerID: 1, Address: "127.0.0.1", Port: 9037, Name: "testserver"}
	if server != expServer {
		t.Fail()
	}

	removeErr := db.RemoveServer(testDB, server.Address, server.Port)
	if removeErr != nil {
		t.Error(removeErr)
	}

	server, getErr = db.GetServer(testDB, "127.0.0.1", 9037)
	if getErr == nil {
		t.Fail()
	}
}
