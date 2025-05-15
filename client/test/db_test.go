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
	// Insert test
	_, saveErr := db.SaveServer(testDB, "127.0.0.1", 9037, "testserver")
	if saveErr != nil {
		t.Error(saveErr)
	}

	// Select test
	actual, getErr := db.GetServer(testDB, "127.0.0.1", 9037)
	if getErr != nil {
		t.Error(getErr)
	}

	expected := db.Server{ServerID: 1, Address: "127.0.0.1", Port: 9037, Name: "testserver"}
	if actual != expected {
		t.Fail()
	}

	// Deletion test
	removeErr := db.RemoveServer(testDB, actual.Address, actual.Port)
	if removeErr != nil {
		t.Error(removeErr)
	}

	actual, getErr = db.GetServer(testDB, "127.0.0.1", 9037)
	if getErr == nil {
		t.Fail()
	}
}

func TestUser(t *testing.T) {
	testDB := setup(t)
	sv, _ := db.SaveServer(testDB, "127.0.0.1", 9037, "testserver")

	// Local user test
	_, addErr := db.AddLocalUser(testDB, "Alice", "test", "test", sv.ServerID)
	if addErr != nil {
		t.Error(addErr)
	}

	actualLocal, getErr := db.GetLocalUser(testDB, "Alice", sv.ServerID)
	if getErr != nil {
		t.Error(getErr)
	}

	expectedLocal := db.LocalUser{UserID: 1, Password: "test", PrvKey: "test", User: db.User{ServerID: sv.ServerID, Username: "Alice"}}

	if expectedLocal == actualLocal {
		t.Fail()
	}

	// External user test
	_, addErr = db.AddExternalUser(testDB, "Bob", "test", sv.ServerID)
	if addErr != nil {
		t.Error(addErr)
	}

	actualExternal, externalGetErr := db.GetExternalUser(testDB, "Bob", sv.ServerID)
	if externalGetErr != nil {
		t.Error(getErr)
	}

	expectedExternal := db.ExternalUser{UserID: 2, PubKey: "test", User: db.User{UserID: 2, ServerID: sv.ServerID, Username: "Bob"}}

	if expectedExternal == actualExternal {
		t.Fail()
	}
}
