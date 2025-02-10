package main

import (
	"context"
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

// Table used for hub values
type table[T any] struct {
	mut sync.Mutex
	tab map[net.Conn]T
}

// Specifies a verification in process
type Verif struct {
	name   username
	text   string
	cancel context.CancelFunc
}

// Determines a request to be processed by a hug
type Request struct {
	cl  net.Conn
	cmd *gc.Command
}

// Specifies a logged in user
type User struct {
	conn   net.Conn
	name   username
	pubkey *rsa.PublicKey
}

// Specifies a message to be received
type Message struct {
	sender  username
	message string
	stamp   int64
}

// Uses a mutex since functions are running concurrently
type Hub struct {
	req    chan Request
	clean  chan net.Conn
	db     *sql.DB
	users  table[*User]
	verifs table[*Verif]
	// TODO: runners here
}

// Identifies a runner for concurrency
type Runner[T any] interface {
	chan T
	Run()
}

/* INTERNAL ERRORS */

var ErrorDeregistered error = errors.New("user has been deregistered")
var ErrorDoesNotExist error = errors.New("data does not exist")
var ErrorSessionExists error = errors.New("user is already logged in")
var ErrorDuplicatedSession error = errors.New("user is logged in from another endpoint")
var ErrorProhibitedOperation error = errors.New("operation trying to be performed is invalid")
var ErrorNoAccount error = errors.New("user tried performing an operation with no account")
var ErrorDBConstraint error = errors.New("database returned constraint on operation")
var ErrorNoMessages error = errors.New("user has no messages to receive")

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
