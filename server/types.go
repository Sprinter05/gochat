package main

import (
	"crypto/rsa"
	"database/sql"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* TYPE DEFINITIONS */

// Has to be used with net.Conn.RemoteAddr().String()
type ip string

// Has to conform to UsernameSize
type username string

// Specifies the functions to run depending on the ID
type actions func(*Hub, *User, gc.Command)

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
	db     *sql.DB
	umut   sync.Mutex
	users  map[ip]*User
	vmut   sync.Mutex
	verifs map[ip]string
}

/* AUXILIARY FUNCTIONS */

// Help with packet creation by logging
func sendErrorPacket(id gc.ID, err error, cl net.Conn) {
	pak, e := gc.NewPacket(gc.ERR, id, gc.ErrorCode(err), nil)
	if e != nil {
		//* Error when creating packet
		log.Println(e)
	} else {
		cl.Write(pak)
	}
}

// Help with packet creation by logging
func sendOKPacket(id gc.ID, cl net.Conn) {
	pak, e := gc.NewPacket(gc.ERR, id, gc.EmptyInfo, nil)
	if e != nil {
		//* Error when creating packet
		log.Println(e)
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
