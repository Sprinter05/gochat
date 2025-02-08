package main

import (
	"crypto/rsa"
	"database/sql"
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* TYPE DEFINITIONS */

// Has to conform to UsernameSize
type username string

// Specifies the functions to run depending on the ID
type actions func(*Hub, *User, gc.Command)

// Specifies a verification in process
type Verif struct {
	name username
	text string
}

// Determines a request to be processed by a hug
type Request struct {
	cl  net.Conn
	cmd gc.Command
}

// Specifies a logged in user
type User struct {
	conn   net.Conn
	name   username
	pubkey *rsa.PublicKey
}

// Uses a mutex since functions are running concurrently
type Hub struct {
	req    chan Request
	clean  chan net.Conn
	db     *sql.DB
	umut   sync.Mutex
	users  map[net.Conn]*User
	vmut   sync.Mutex
	verifs map[net.Conn]*Verif
}

/* INTERNAL ERRORS */

var ErrorDeregistered error = errors.New("user has been deregistered")
var ErrorDoesNotExist error = errors.New("data does not exist")
var ErrorSessionExists error = errors.New("user is already logged in")
var ErrorDuplicatedSession error = errors.New("user is logged in from another endpoint")
var ErrorProhibitedOperation error = errors.New("operation trying to be performed is invalid")
var ErrorNoAccount error = errors.New("user tried performing an operation with no account")

/* AUXILIARY FUNCTIONS */

// Help with packet creation by logging
func sendErrorPacket(id gc.ID, err error, cl net.Conn) {
	pak, e := gc.NewPacket(gc.ERR, id, gc.ErrorCode(err), nil)
	if e != nil {
		log.Printf("Error when creating ERR packet: %s\n", e)
	} else {
		cl.Write(pak)
	}
}

// Help with packet creation by logging
func sendOKPacket(id gc.ID, cl net.Conn) {
	pak, e := gc.NewPacket(gc.OK, id, gc.EmptyInfo, nil)
	if e != nil {
		log.Printf("Error when creating OK packet: %s\n", e)
	} else {
		cl.Write(pak)
	}
}

// Generate a random text
func randText() []byte {
	// Set seed
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	set := []byte(gc.CypherCharset)

	// Generate random characters
	r := make([]byte, gc.CypherLength)
	for i := range r {
		r[i] = set[seed.Intn(len(gc.CypherCharset))]
	}

	return r
}
