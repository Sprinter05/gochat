package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
	"gorm.io/gorm"
)

/* TYPE DEFINITIONS */

// Cypher values
const CypherCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"
const CypherLength int = 128

// Has to conform to UsernameSize on the specification
type username string

// Specifies the functions to run depending on the ID
type action func(*Hub, User, gc.Command)

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

// Determines a request to be processed by a thread
type Request struct {
	cl  net.Conn
	cmd gc.Command
}

const MaxUserRequests int = 5

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

// Tables store pointers for modification
// But functions should not use the pointer
type Hub struct {
	db     *gorm.DB
	clean  chan net.Conn
	shtdwn chan bool
	users  table[*User]
	verifs table[*Verif]
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
	ErrorCLIArgs             error = errors.New("no CLI argument provided")
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
	i := 0

	t.mut.RLock()
	defer t.mut.RUnlock()
	for _, v := range t.tab {
		array[i] = v
		i++
	}

	return array
}

/* AUXILIARY FUNCTIONS */

// Wrap the error sending function
func sendErrorPacket(id gc.ID, err error, cl net.Conn) {
	pak, e := gc.NewPacket(gc.ERR, id, gc.ErrorCode(err), nil)
	if e != nil {
		gclog.Packet(gc.ERR, e)
	} else {
		cl.Write(pak)
	}
}

// Wrap the acknowledgement sending function
func sendOKPacket(id gc.ID, cl net.Conn) {
	pak, e := gc.NewPacket(gc.OK, id, gc.EmptyInfo, nil)
	if e != nil {
		gclog.Packet(gc.OK, e)
	} else {
		cl.Write(pak)
	}
}

// Generate a random text using the specification charset
func randText() []byte {
	// Set seed in nanoseconds for better randomness
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	set := []byte(CypherCharset)

	r := make([]byte, CypherLength)
	for i := range r {
		r[i] = set[seed.Intn(len(CypherCharset))]
	}

	return r
}
