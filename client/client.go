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

// Creates a user given its username, generating a key pair for it
// ! No devuelvas un pointer si en la funcion que lo usa lo vas a dereferenciar
// ! Cada vez q mueves un pointer activas el garbage collector, asi que evitalo en la medida de lo posible
// ! Solo deberias usar pointers si es para modificar algo
func NewUser(username string) (*Client, error) {
	// Generates a key pair
	pair, genErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if genErr != nil {
		return nil, genErr
	}
	// Stores the pubkey's bytes to be stored in the database
	pubKey, _ := spec.PubkeytoPEM(&pair.PublicKey)
	// ! Evita comentarios superfluos que no aportan nada
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
