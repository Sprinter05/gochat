package hubs

import (
	"bytes"
	"context"
	"crypto/rsa"
	"net"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
)

/* TYPES */

// Specifies a user that is connected/online.
// By design it is not safe to use concurrently,
// but it depends on how is is being used.
type User struct {
	conn   net.Conn       // TCP Connection
	secure bool           // Whether it is using TLS or not
	name   string         // Username, must conform to the specification size
	perms  db.Permission  // Level of permission
	pubkey *rsa.PublicKey // Public RSA key
}

// Specifies a verification in process or
// a reusable token. It is not safe to use
// concurrently but it depends on how it is being used.
type Verif struct {
	conn    net.Conn           // TCP Connection
	name    string             // Username, must conform to the specification size
	text    []byte             // Random text in unencrypted state
	pending bool               // If false, it is in reusable token state
	cancel  context.CancelFunc // Function to stop the pending verification
	expiry  time.Time          // How long it is available for after a disconnection
}

/* USER FUNCTIONS */

// Queries and transforms a user from the database into
// a hub user that is online. It also checks that the retrieved
// user does not have malformed data or that it hasn't
// become deregistered.
//
// Returns a specification error.
func (hub *Hub) userFromDB(uname string) (*User, error) {
	dbuser, err := db.QueryUser(hub.db, uname)
	if err != nil {
		if err == db.ErrorNotFound {
			return nil, spec.ErrorNotFound
		}

		return nil, spec.ErrorServer
	}

	// Check that the permission int is not out of bounds
	if dbuser.Permission > db.OWNER {
		return nil, spec.ErrorCorrupted
	}

	// Check that the public key is not null
	if !dbuser.Pubkey.Valid {
		return nil, spec.ErrorDeregistered
	}

	// Turn it into a public key from PEM certificate
	key, err := spec.PEMToPubkey([]byte(dbuser.Pubkey.String))
	if err != nil {
		return nil, spec.ErrorCorrupted
	}

	// Connection remains null as we don't know if it will be online
	// Should be assigned by the calling function if necessary
	// Connection is also by default not secure because its not connected
	return &User{
		conn:   nil,
		secure: false,
		name:   uname,
		pubkey: key,
		perms:  dbuser.Permission,
	}, nil
}

// Checks if a reusable token is applicable to a user and if
// it is valid and safe to use.
//
// Returns a specification error.
func (hub *Hub) checkToken(u User, text []byte) error {
	if !u.secure {
		// We do not remove the verif
		// This allows trying again with a secure connection
		return spec.ErrorUnsecure
	}

	v, ok := hub.verifs.Get(u.name)
	if !ok {
		return spec.ErrorNotFound
	}

	if v.pending {
		return spec.ErrorInvalid
	}

	// Check if it has expired
	if time.Until(v.expiry) <= 0 {
		hub.verifs.Remove(u.name)
		return spec.ErrorNotFound
	}

	if !bytes.Equal(v.text, text) {
		return spec.ErrorHandshake
	}

	return nil
}

/* EXPORTED FUNCTIONS */

// Tries to find an online user, returning a boolean
// that indicates if it was found or not.
func (hub *Hub) FindUser(uname string) (*User, bool) {
	list := hub.users.GetAll()
	for _, v := range list {
		if v.name == uname {
			return v, true
		}
	}

	return nil, false
}

// Provides a list of all registered users, if the "online"
// parameter is true, it will only return online users.
// If no results are found, an empty string will be returned.
func (hub *Hub) Userlist(online bool) (ret string) {
	if online {
		var str strings.Builder
		list := hub.users.GetAll()

		// Preallocate strings builder
		for _, v := range list {
			str.Grow(len(v.name))
		}

		for _, v := range list {
			str.WriteString(string(v.name) + "\n")
		}

		l := str.Len()
		ret = str.String()

		// Remove the last newline
		ret = ret[:l-1]
	} else {
		query, err := db.QueryUsernames(hub.db)
		if err != nil {
			log.DB("userlist", err)
		}
		ret = query
	}

	// Will return "" if nothing is found
	return ret
}
