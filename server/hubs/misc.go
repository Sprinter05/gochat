package hubs

import (
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
