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

// Has to conform to UsernameSize on the specification
type username string

// Specifies the functions to run depending on the ID
type action func(*Hub, *User, gc.Command)

// Table used for storing thread safe maps
type table[T any] struct {
	// TODO: RWMutex
	mut sync.Mutex
	tab map[net.Conn]T
}

// Determines a request to be processed by a hub
type Request struct {
	cl  net.Conn
	cmd gc.Command
}

// Specifies a task to be performed by a runner
type Task struct {
	fun  action
	hub  *Hub
	user *User
	cmd  gc.Command
}

// Specifies a logged in user
type User struct {
	conn   net.Conn
	name   username
	pubkey *rsa.PublicKey
}

// Specifies a verification in process
type Verif struct {
	name   username
	text   string
	cancel context.CancelFunc
}

// Specifies a message to be received
type Message struct {
	sender  username
	message string
	stamp   int64
}

// Uses a mutex since functions are running concurrently
type Hub struct {
	db      *sql.DB
	req     chan Request
	clean   chan net.Conn
	users   table[*User]
	verifs  table[*Verif]
	runners table[chan Task]
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

/* TABLE FUNCTIONS */

func (t *table[T]) Add(i net.Conn, v T) {
	t.mut.Lock()
	t.tab[i] = v
	t.mut.Unlock()
}

func (t *table[T]) Remove(i net.Conn) {
	t.mut.Lock()
	delete(t.tab, i)
	t.mut.Unlock()
}

func (t *table[T]) Get(i net.Conn) (T, bool) {
	look := t.tab
	t.mut.Lock()
	v, ok := look[i]
	t.mut.Unlock()

	if !ok {
		var zero T // Empty value of T
		return zero, false
	}

	return v, true
}

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
