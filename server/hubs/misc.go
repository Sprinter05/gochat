package hubs

import (
	"errors"
	"math/rand"
	"net"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* CONSTANTS */

// Cypher values
const CypherCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"
const CypherLength int = 128

// Used for the size of the queue of requests
const MaxUserRequests int = 5

/* INTERNAL ERRORS */

var (
	ErrorDeregistered        error = errors.New("user has been deregistered")
	ErrorDoesNotExist        error = errors.New("data does not exist")
	ErrorSessionExists       error = errors.New("user is already logged in")
	ErrorDuplicatedSession   error = errors.New("user is logged in from another endpoint")
	ErrorProhibitedOperation error = errors.New("operation trying to be performed is invalid")
	ErrorNoAccount           error = errors.New("user tried performing an operation with no account")
	ErrorNoMessages          error = errors.New("user has no messages to receive")
	ErrorInvalidValue        error = errors.New("data provided is invalid")
)

/* AUXILIARY FUNCTIONS */

// Catches up messages for the logged connection
func catchUp(cl net.Conn, id spec.ID, msgs ...*spec.Message) error {
	for _, v := range msgs {
		// Turn timestamp to byte array and create packet
		stp := spec.UnixStampToBytes(v.Stamp)

		pak, err := spec.NewPacket(spec.RECIV, id, spec.EmptyInfo,
			[]byte(v.Sender),
			stp,
			v.Content,
		)
		if err != nil {
			log.Packet(spec.RECIV, err)
			return err
		}
		cl.Write(pak)
	}

	return nil
}

// Wrap the error sending function
func sendErrorPacket(id spec.ID, err error, cl net.Conn) {
	pak, e := spec.NewPacket(spec.ERR, id, spec.ErrorCode(err))
	if e != nil {
		log.Packet(spec.ERR, e)
	} else {
		cl.Write(pak)
	}
}

// Wrap the acknowledgement sending function
func sendOKPacket(id spec.ID, cl net.Conn) {
	pak, e := spec.NewPacket(spec.OK, id, spec.EmptyInfo)
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
