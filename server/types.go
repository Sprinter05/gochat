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
	mut sync.RWMutex
	tab map[net.Conn]T
}

// Specifies a permission
type Permission int8

const (
	USER Permission = iota
	ADMIN
	OWNER
)

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
	perms  Permission
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

var (
	ErrorDeregistered        error = errors.New("user has been deregistered")
	ErrorDoesNotExist        error = errors.New("data does not exist")
	ErrorSessionExists       error = errors.New("user is already logged in")
	ErrorDuplicatedSession   error = errors.New("user is logged in from another endpoint")
	ErrorProhibitedOperation error = errors.New("operation trying to be performed is invalid")
	ErrorNoAccount           error = errors.New("user tried performing an operation with no account")
	ErrorDBConstraint        error = errors.New("database returned constraint on operation")
	ErrorNoMessages          error = errors.New("user has no messages to receive")
	ErrorInvalidValue        error = errors.New("data provided is invalid")
)

/* TABLE FUNCTIONS */

// Thread safe write
func (t *table[T]) Add(i net.Conn, v T) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.tab[i] = v
}

// Thread safe write
func (t *table[T]) Remove(i net.Conn) {
	t.mut.Lock()
	defer t.mut.Unlock()
	delete(t.tab, i)
}

// Thread safe read
func (t *table[T]) Get(i net.Conn) (T, bool) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	v, ok := t.tab[i]

	if !ok {
		// Empty value of T
		var empty T
		return empty, false
	}

	return v, true
}

// Thread safe read
func (t *table[T]) GetAll() []T {
	l := len(t.tab)
	if l == 0 {
		return nil
	}

	array := make([]T, l)

	t.mut.RLock()
	defer t.mut.RUnlock()
	for _, v := range t.tab {
		array = append(array, v)
	}

	return array
}

/* AUXILIARY FUNCTIONS */

// Wrap the error sending function
func sendErrorPacket(id gc.ID, err error, cl net.Conn) {
	pak, e := gc.NewPacket(gc.ERR, id, gc.ErrorCode(err), nil)
	if e != nil {
		log.Printf("Error when creating ERR packet: %s\n", e)
	} else {
		cl.Write(pak)
	}
}

// Wrap the acknowledgement sending function
func sendOKPacket(id gc.ID, cl net.Conn) {
	pak, e := gc.NewPacket(gc.OK, id, gc.EmptyInfo, nil)
	if e != nil {
		log.Printf("Error when creating OK packet: %s\n", e)
	} else {
		cl.Write(pak)
	}
}

// Generate a random text using the specification charset
func randText() []byte {
	// Set seed in nanoseconds for better randomness
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	set := []byte(gc.CypherCharset)

	r := make([]byte, gc.CypherLength)
	for i := range r {
		r[i] = set[seed.Intn(len(gc.CypherCharset))]
	}

	return r
}
