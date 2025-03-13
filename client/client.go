package main

import (
	"crypto/rand"
	"crypto/rsa"

	gc "github.com/Sprinter05/gochat/internal/spec"
)

type Client struct {
	username string
	keyPair  *rsa.PrivateKey
}

// Creates a user given its username, generating a key pair for it
func NewUser(username string) (*Client, error) {
	// Generates a key pair
	pair, genErr := rsa.GenerateKey(rand.Reader, gc.RSABitSize)
	if genErr != nil {
		return nil, genErr
	}
	// Stores the pubkey's bytes to be stored in the database
	pubKey, _ := gc.PubkeytoPEM(&pair.PublicKey)
	// Creates the client
	client := Client{username: username, keyPair: pair}
	// Adds it to the database
	dbErr := AddUser(username, string(pubKey))
	if dbErr != nil {
		return nil, dbErr
	}
	// Only if there are no errors is the client returned
	return &client, nil
}
