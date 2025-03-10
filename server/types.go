package main

import (
	"context"
	"crypto/rsa"
	"math/rand"
	"net"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/model"
	"gorm.io/gorm"
)

/* CONSTANTS */

// Cypher values
const CypherCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"
const CypherLength int = 128

// Used for the size of the queue of requests
const MaxUserRequests int = 5

/* TYPE DEFINITIONS */

// Specifies the functions to run depending on the ID
type action func(*Hub, User, spec.Command)

// Determines a request to be processed by a thread
type Request struct {
	cl  net.Conn
	cmd spec.Command
	tls bool
}

// Specifies a logged in user
type User struct {
	conn   net.Conn
	secure bool
	name   model.Username
	perms  model.Permission
	pubkey *rsa.PublicKey
}

// Specifies a verification in process
// Can also be used for reusable tokens
type Verif struct {
	conn    net.Conn
	name    model.Username
	text    string
	pending bool
	cancel  context.CancelFunc
	expiry  time.Time
}

// Tables store pointers for modification
// But functions should not use the pointer
type Hub struct {
	db     *gorm.DB
	clean  chan net.Conn
	shtdwn chan bool
	users  model.Table[net.Conn, *User]
	verifs model.Table[model.Username, *Verif]
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
