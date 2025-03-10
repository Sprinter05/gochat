package main

import (
	"net"
	"os"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/model"
)

/* HUB WRAPPER FUNCTIONS */

// Returns a user struct by querying the database one
func (hub *Hub) GetUserFromDB(uname model.Username) (*User, error) {
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

// Cleans any mention to a connection in the caches
func (h *Hub) CleanupUser(cl net.Conn) {
	// Cleanup on the users table
	h.users.Remove(cl)

	// Cleanup on the verification table
	list := h.verifs.GetAll()
	for _, v := range list {
		if v.conn == cl {
			h.verifs.Remove(v.name)
			// If not pending we assume the connection was secure
			if !v.pending {
				// If not in verif we readd it with nil connection
				v.conn = nil
				v.expiry = time.Now().Add(
					time.Duration(spec.TokenExpiration) * time.Minute,
				)
				h.verifs.Add(v.name, v)
			}
		}
	}
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) CheckSession(r Request) (*User, error) {
	op := r.cmd.HD.Op

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
			sendErrorPacket(r.cmd.HD.ID, spec.ErrorNotFound, r.cl)
		} else {
			// Cannot do anything without an account
			sendErrorPacket(r.cmd.HD.ID, spec.ErrorNoSession, r.cl)
		}
		return nil, model.ErrorNoAccount
	}

	// Newly created user
	// The REG function is expected to fill the rest of the struct
	// Its thread safe because the pointer is not yet in the cache
	return &User{
		conn: r.cl,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Check if there is a user entry from the database
func (h *Hub) dbLogin(r Request) (*User, error) {
	// Check if the user is in the database
	u := model.Username(r.cmd.Args[0])
	user, e := h.GetUserFromDB(u)
	if e != nil {
		// Error is handled in the calling function
		if e != model.ErrorDoesNotExist {
			sendErrorPacket(r.cmd.HD.ID, spec.ErrorLogin, r.cl)
		}
		return nil, e
	}

	// Assign connection and if said connection is secure
	user.conn = r.cl
	user.secure = r.tls
	return user, nil
}

// Check if the user is already logged in from the cache
func (h *Hub) cachedLogin(r Request) (*User, error) {
	id := r.cmd.HD.Op

	v, ok := h.users.Get(r.cl)
	if ok {
		// User is cached and the session can be returned
		return v, nil
	}

	if id == spec.LOGIN {
		// We check if the user is logged in from another IP
		_, ipok := h.FindUser(model.Username(r.cmd.Args[0]))
		if ipok {
			// Cannot have two sessions of the same user
			sendErrorPacket(r.cmd.HD.ID, spec.ErrorLogin, r.cl)
			return nil, model.ErrorDuplicatedSession

		}
	}

	// Otherwise the user is not found
	return nil, model.ErrorDoesNotExist
}

/* HUB QUERY FUNCTIONS */

// Lists all users in the server
func (h *Hub) Userlist(online bool) string {
	var str strings.Builder
	var ret string = ""
	var err error

	if online {
		list := h.users.GetAll()
		for _, v := range list {
			str.WriteString(string(v.name) + "\n")
		}

		l := str.Len()
		ret = str.String()

		// Remove the last newline
		ret = ret[:l-1]
	} else {
		ret, err = db.QueryUsernames(h.db)
		if err != nil {
			log.DB("userlist", err)
		}
	}

	// Will return "" if nothing is found
	return ret
}

// Returns an online user if it exists (thread-safe)
func (h *Hub) FindUser(uname model.Username) (*User, bool) {
	list := h.users.GetAll()
	for _, v := range list {
		if v.name == uname {
			return v, true
		}
	}

	return nil, false
}

// Checks if a reusable token is valid
func (h *Hub) CheckToken(u User, text string) error {
	if !u.secure {
		// We do not remove the verif
		// This allows trying again with a secure connection
		return spec.ErrorUnescure
	}

	v, ok := h.verifs.Get(u.name)
	if !ok {
		return spec.ErrorNotFound
	}

	if v.pending {
		return spec.ErrorInvalid
	}

	// Check if it has expired
	if time.Until(v.expiry) <= 0 {
		h.verifs.Remove(u.name)
		return spec.ErrorNotFound
	}

	if v.text != text {
		return spec.ErrorHandshake
	}

	return nil
}

/* HUB MAIN */

// Handles generic server functions
func (hub *Hub) Start() {
	// Allocate tables with the max clients we may have
	hub.users.Init(spec.MaxClients)
	hub.verifs.Init(spec.MaxClients)

	for {
		select {
		case <-hub.shtdwn:
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
			os.Exit(0)
		case c := <-hub.clean:
			// Remove all mentions of the user in the cache
			hub.CleanupUser(c)
		}
	}
}
