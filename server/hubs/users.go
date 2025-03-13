package hubs

import (
	"bytes"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
)

/* CHECK FUNCTIONS */

// Returns a user struct by querying the database one
func (hub *Hub) userFromDB(uname string) (*User, error) {
	dbuser, err := db.QueryUser(hub.db, uname)
	if err != nil {
		return nil, err
	}

	// Check that the permissions int is not out of bounds
	if dbuser.Permission > db.OWNER {
		return nil, spec.ErrorServer
	}

	// Check that the public key is not null
	if !dbuser.Pubkey.Valid {
		return nil, spec.ErrorDeregistered
	}

	// Turn it into a public key from PEM certificate
	key, err := spec.PEMToPubkey([]byte(dbuser.Pubkey.String))
	if err != nil {
		return nil, err
	}

	// Connection remains null as we don't know if it will be online
	// Should be assigned by the calling function if necessary
	// Connection is also not secure because its not connected
	return &User{
		conn:   nil,
		secure: false,
		name:   uname,
		pubkey: key,
		perms:  dbuser.Permission,
	}, nil
}

// Checks if a reusable token is valid
func (hub *Hub) checkToken(u User, text []byte) error {
	if !u.secure {
		// We do not remove the verif
		// This allows trying again with a secure connection
		return spec.ErrorUnescure
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

// Returns an online user if it exists (thread-safe)
func (hub *Hub) FindUser(uname string) (*User, bool) {
	list := hub.users.GetAll()
	for _, v := range list {
		if v.name == uname {
			return v, true
		}
	}

	return nil, false
}

// Lists all users in the server
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
