package hubs

import (
	"bytes"
	"net"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/model"
	"gorm.io/gorm"
)

/* HUB WRAPPER FUNCTIONS */

// Returns a user struct by querying the database one
func (hub *Hub) userFromDB(uname string) (*User, error) {
	dbuser, err := db.QueryUser(hub.db, uname)
	if err != nil {
		return nil, err
	}

	// Check that the permissions int is not out of bounds
	if dbuser.Permission > model.OWNER {
		return nil, model.ErrorInvalidValue
	}

	// Check that the public key is not null
	if !dbuser.Pubkey.Valid {
		return nil, model.ErrorDeregistered
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

/* HUB HELPER FUNCTIONS */

// Cleans any mention to a connection in the caches
func (hub *Hub) Cleanup(cl net.Conn) {
	// Cleanup on the users table
	hub.users.Remove(cl)

	// Cleanup on the verification table
	list := hub.verifs.GetAll()
	for _, v := range list {
		if v.conn == cl {
			hub.verifs.Remove(v.name)
			// If not pending we assume the connection was secure
			if !v.pending {
				// If not in verif we readd it with nil connection
				v.conn = nil
				v.expiry = time.Now().Add(
					time.Duration(spec.TokenExpiration) * time.Minute,
				)
				hub.verifs.Add(v.name, v)
			}
		}
	}
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) Session(r Request) (*User, error) {
	op := r.Command.HD.Op

	// Can not be REG LOGIN or VERIF if checking in the cache
	if op != spec.REG && op != spec.LOGIN && op != spec.VERIF {
		cached, err := hub.cachedLogin(r)
		if err == nil {
			// Valid user found in cache, serve request
			return cached, nil
		} else if err != model.ErrorDoesNotExist {
			// We do not search in the DB if its a different error
			return nil, err
		}
	}

	// Can only be LOGIN or VERIF if checking in the database
	if op == spec.LOGIN || op == spec.VERIF {
		user, e := hub.dbLogin(r)
		if e == nil {
			// User found in database so we serve request
			return user, nil
		} else if e != model.ErrorDoesNotExist {
			// We do not create a new user if its a different error
			return nil, e
		}
	}

	// Can only be REG if no user was found
	// So if its not REG we error
	if op != spec.REG {
		if op == spec.LOGIN {
			// User does not exist when trying to login
			sendErrorPacket(r.Command.HD.ID, spec.ErrorNotFound, r.Conn)
		} else {
			// Cannot do anything without an account
			sendErrorPacket(r.Command.HD.ID, spec.ErrorNoSession, r.Conn)
		}
		return nil, model.ErrorNoAccount
	}

	// Newly created user
	// The REG function is expected to fill the rest of the struct
	// Its thread safe because the pointer is not yet in the cache
	return &User{
		conn: r.Conn,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Check if there is a user entry from the database
func (hub *Hub) dbLogin(r Request) (*User, error) {
	// Check if the user is in the database
	u := string(r.Command.Args[0])
	user, e := hub.userFromDB(u)
	if e != nil {
		// Error is handled in the calling function
		if e != model.ErrorDoesNotExist {
			sendErrorPacket(r.Command.HD.ID, spec.ErrorLogin, r.Conn)
		}
		return nil, e
	}

	// Assign connection and if said connection is secure
	user.conn = r.Conn
	user.secure = r.TLS
	return user, nil
}

// Check if the user is already logged in from the cache
func (hub *Hub) cachedLogin(r Request) (*User, error) {
	id := r.Command.HD.Op

	v, ok := hub.users.Get(r.Conn)
	if ok {
		// User is cached and the session can be returned
		return v, nil
	}

	if id == spec.LOGIN {
		// We check if the user is logged in from another IP
		_, ipok := hub.FindUser(string(r.Command.Args[0]))
		if ipok {
			// Cannot have two sessions of the same user
			sendErrorPacket(r.Command.HD.ID, spec.ErrorLogin, r.Conn)
			return nil, model.ErrorDuplicatedSession

		}
	}

	// Otherwise the user is not found
	return nil, model.ErrorDoesNotExist
}

/* HUB QUERY FUNCTIONS */

// Lists all users in the server
func (hub *Hub) Userlist(online bool) string {
	var str strings.Builder
	var ret string
	var err error

	if online {
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
		ret, err = db.QueryUsernames(hub.db)
		if err != nil {
			log.DB("userlist", err)
		}
	}

	// Will return "" if nothing is found
	return ret
}

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

/* HUB MAIN */

// Initialises all data structures
func Create(database *gorm.DB) *Hub {
	hub := &Hub{
		clean:  make(chan net.Conn, spec.MaxClients/2),
		shtdwn: make(chan bool),
		users:  model.NewTable[net.Conn, *User](spec.MaxClients),
		verifs: model.NewTable[string, *Verif](spec.MaxClients),
		db:     database,
	}

	return hub
}

// Waits until the hub has to shutdown
func (hub *Hub) Wait() {
	// Wait until the shutdown happens
	for range hub.shtdwn {
		// Disconnect all users
		list := hub.users.GetAll()
		for _, v := range list {
			// This should trigger the cleanup function too
			v.conn.Close()
		}

		// Wait a bit for everything to close
		time.Sleep(5 * time.Second)

		// Perform a server shutdown
		log.Notice("inminent server shutdown")
	}
}
