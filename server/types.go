package main

import (
	"context"
	"crypto/rsa"
	"errors"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
)

/* TYPE DEFINITIONS */

// Cypher values
const CypherCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"
const CypherLength int = 128

// Has to conform to UsernameSize on the specification
type username string

// Specifies the functions to run depending on the ID
type action func(*Hub, User, spec.Command)

// Table used for storing thread safe maps
type table[I comparable, T any] struct {
	mut sync.RWMutex
	tab map[I]T
}

// Global counter for the amount of clients
type Counter struct {
	mut sync.Mutex
	val int
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
	cmd spec.Command
	tls bool
}

const MaxUserRequests int = 5

// Specifies a logged in user
type User struct {
	conn   net.Conn
	secure bool
	name   username
	perms  Permission
	pubkey *rsa.PublicKey
}

// Specifies a verification in process
type Verif struct {
	conn    net.Conn
	name    username
	text    string
	pending bool
	cancel  context.CancelFunc
	expiry  time.Time
}

// Specifies a message to be received
type Message struct {
	sender  username
	message []byte
	stamp   time.Time
}

// Tables store pointers for modification
// But functions should not use the pointer
type Hub struct {
	db     *gorm.DB
	clean  chan net.Conn
	shtdwn chan bool
	users  table[net.Conn, *User]
	verifs table[username, *Verif]
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
func (t *table[I, T]) Add(i I, v T) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.tab[i] = v
}

// Thread safe write
func (t *table[I, T]) Remove(i I) {
	t.mut.Lock()
	defer t.mut.Unlock()
	delete(t.tab, i)
}

// Thread safe read
func (t *table[I, T]) Get(i I) (T, bool) {
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
func (t *table[I, T]) GetAll() []T {
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

/* COUNTER FUNCTIONS */

func (c *Counter) Get() int {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.val
}

func (c *Counter) Inc() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.val++
}

func (c *Counter) Dec() {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.val > 0 {
		c.val--
	}
}

/* AUXILIARY FUNCTIONS */

// Wrap the error sending function
func sendErrorPacket(id spec.ID, err error, cl net.Conn) {
	pak, e := spec.NewPacket(spec.ERR, id, spec.ErrorCode(err), nil)
	if e != nil {
		log.Packet(spec.ERR, e)
	} else {
		cl.Write(pak)
	}
}

// Wrap the acknowledgement sending function
func sendOKPacket(id spec.ID, cl net.Conn) {
	pak, e := spec.NewPacket(spec.OK, id, spec.EmptyInfo, nil)
	if e != nil {
		log.Packet(spec.OK, e)
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
