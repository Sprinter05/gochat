package test

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHash(t *testing.T) {
	pass := "password123"
	// Hashes password
	hash, hashErr := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr != nil {
		t.Fatal(hashErr)
	}
	// Verifies hash
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		t.Fail()
	}
}
func TestSalt(t *testing.T) {
	pass := "password123"
	// Generates two hashes for the password
	hash1, hashErr1 := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr1 != nil {
		t.Fatal(hashErr1)
	}
	hash2, hashErr2 := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr2 != nil {
		t.Fatal(hashErr2)
	}
	// Fails the test if they're equal. If they are not, it means the
	// hashes are indeed being salted
	if string(hash1) == string(hash2) {
		t.Fail()
	}
	// Verifies that both hashes do correspond to the password
	cmpErr1 := bcrypt.CompareHashAndPassword(hash1, []byte(pass))
	if cmpErr1 != nil {
		t.Fail()
	}

	cmpErr2 := bcrypt.CompareHashAndPassword(hash2, []byte(pass))
	if cmpErr2 != nil {
		t.Fail()
	}
}
