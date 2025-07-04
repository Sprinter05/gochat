package hubs

import (
	"math/rand"
	"net"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* CONSTANTS */

// Charset to be used by the random text generator
const randTextCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"

// Amount of characters the random text should have
const randTextLength int = 128

/* AUXILIARY FUNCTIONS */

// Removes a use from all hooks that exist, mainly
// for the purpose of cleaning up the connection.
func removeFromHooks(h *Hub, cl net.Conn) {
	hooks := h.subs.GetAll()

	for _, hook := range hooks {
		list := hook.Copy(0)
		for _, v := range list {
			if v == cl {
				hook.Remove(v)
			}
		}
	}
}

// Auxiliary function that sends all messages that were retrieved from
// the database to the recently connected user. This function does not
// touch the database, it just sends the messages.
func catchUp(cl net.Conn, msgs ...*spec.Message) {
	for _, v := range msgs {
		// Turn timestamp to byte array and create packet
		stp := spec.UnixStampToBytes(v.Stamp)

		pak, err := spec.NewPacket(spec.RECIV, spec.NullID, spec.EmptyInfo,
			[]byte(v.Sender),
			stp,
			v.Content,
		)

		if err != nil {
			log.Packet(spec.RECIV, err)
		}

		cl.Write(pak)
	}
}

// Auxiliary function to reduce code when sending errors.
func SendErrorPacket(id spec.ID, err error, cl net.Conn) {
	pak, err := spec.NewPacket(spec.ERR, id, spec.ErrorCode(err))
	if err != nil {
		log.Packet(spec.ERR, err)
	} else {
		cl.Write(pak)
	}
}

// Auxiliary function to reduce code when sending ok packets.
func SendOKPacket(id spec.ID, cl net.Conn) {
	pak, err := spec.NewPacket(spec.OK, id, spec.EmptyInfo)
	if err != nil {
		log.Packet(spec.OK, err)
	} else {
		cl.Write(pak)
	}
}

// Generate a random text using a fixed charset and size
// This can be used for verification tokens.
func randText() []byte {
	// Set seed in nanoseconds for better randomness
	seed := rand.New(rand.NewSource(time.Now().UnixNano()))
	set := []byte(randTextCharset)

	r := make([]byte, randTextLength)
	for i := range r {
		r[i] = set[seed.Intn(len(randTextCharset))]
	}

	return r
}
