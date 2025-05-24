package test

import (
	"fmt"
	"testing"
	"time"

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
	_, saveErr := db.SaveServer(testDB, "127.0.0.1", 9037, "testserver", false)
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
	sv, _ := db.SaveServer(testDB, "127.0.0.1", 9037, "testserver", false)

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

func TestMessage(t *testing.T) {
	testDB := setup(t)
	sv, _ := db.AddServer(testDB, db.Server{Address: "127.0.0.0", Port: 9037, ServerID: 1, Name: "Default"})
	src, _ := db.AddLocalUser(testDB, "Alice", "test", "test", sv.ServerID)
	dst, _ := db.AddExternalUser(testDB, "Bob", "test", sv.ServerID)

	_, err := db.StoreMessage(testDB, src.User, dst.User, "hello world", time.Now())
	if err != nil {
		t.Error(err)
	}

	m1 := db.Message{SourceUser: src.User, DestinationUser: dst.User, Stamp: time.Date(2025, 5, 16, 12, 0, 0, 0, time.UTC), Text: "hello world"}
	m2 := db.Message{SourceUser: dst.User, DestinationUser: src.User, Stamp: time.Date(2025, 5, 15, 12, 0, 0, 0, time.UTC), Text: "bye"}
	m3 := db.Message{SourceUser: dst.User, DestinationUser: src.User, Stamp: time.Date(2025, 5, 19, 12, 0, 0, 0, time.UTC), Text: "bye 2"}

	_, err = db.StoreMessage(testDB, m1.SourceUser, m1.DestinationUser, m1.Text, m1.Stamp)
	if err != nil {
		t.Error(err)
	}

	_, err = db.StoreMessage(testDB, m2.SourceUser, m2.DestinationUser, m2.Text, m2.Stamp)
	if err != nil {
		t.Error(err)
	}

	_, err = db.StoreMessage(testDB, m3.SourceUser, m3.DestinationUser, m3.Text, m3.Stamp)
	if err != nil {
		t.Error(err)
	}

	messages, err := db.GetUsersMessagesRange(testDB, src.User, dst.User, time.Date(2025, 5, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Error(err)
	}

	for _, v := range messages {
		fmt.Println(v.Text)
	}

	allMessages, err := db.GetAllUsersMessages(testDB, src.User, dst.User)
	if err != nil {
		t.Error(err)
	}

	for _, v := range allMessages {
		fmt.Println(v.Text)
	}
}
