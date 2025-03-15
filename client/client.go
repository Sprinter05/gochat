package main

import (
	"crypto/rand"
	"crypto/rsa"

	"github.com/Sprinter05/gochat/internal/spec"
)

type Client struct {
	username string
	keyPair  *rsa.PrivateKey
}

// Creates a user given its username, generating a key pair for it. The client data will be nil if an error occurs
func NewUser(username string) (Client, error) {
	// Generates a key pair
	pair, genErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if genErr != nil {
		return Client{username: "", keyPair: nil}, genErr
	}
	// Stores the pubkey's bytes to be stored in the database
	pubKey, _ := spec.PubkeytoPEM(&pair.PublicKey)

	client := Client{username: username, keyPair: pair}
	// Adds it to the database
	dbErr := AddUser(username, string(pubKey))
	if dbErr != nil {
		return Client{username: "", keyPair: nil}, dbErr
	}

	return client, nil
}
