package main

import (
	"net"
	"os"
	"strings"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* HUB WRAPPER FUNCTIONS */

// Cleans any mention to a connection in the caches
func (h *Hub) cleanupUser(cl net.Conn) {
	// Cleanup on the users table
	h.users.Remove(cl)

	// Cleanup on the verification table
	list := h.verifs.GetAll()
	for _, v := range list {
		if v.conn == cl {
			h.verifs.Remove(v.name)
			if !v.pending {
				// If not in verif we readd it with nil connection
				v.conn = nil
				h.verifs.Add(v.name, v)
			}
		}
	}
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) checkSession(r Request) (*User, error) {
	op := r.cmd.HD.Op

	// Cant be REG LOGIN or VERIF
	if op != gc.REG && op != gc.LOGIN && op != gc.VERIF {
		cached, err := hub.cachedLogin(r)
		if err == nil {
			// Valid user found in cache, serve request
			return cached, nil
		} else if err != ErrorDoesNotExist {
			// We do not search in the DB if its a different error
			return nil, err
		}
	}

	// Can only be LOGIN or VERIF
	if op == gc.LOGIN || op == gc.VERIF {
		user, e := hub.dbLogin(r)
		if e == nil {
			// User found in database so we return it
			return user, nil
		} else if e != ErrorDoesNotExist {
			// We do not create a new user if its a different error
			return nil, e
		}
	}

	// Can only be REG
	if op != gc.REG {
		if op == gc.LOGIN {
			// User does not exist when trying to login
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorNotFound, r.cl)
		} else {
			// Cannot do anything without an account
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorNoSession, r.cl)
		}
		return nil, ErrorNoAccount
	}

	// Newly created user
	// The REG function is expected to fill the rest of the struct
	return &User{
		conn: r.cl,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Check if there is a user entry from the database
func (h *Hub) dbLogin(r Request) (*User, error) {

	// Check if the user is in the database
	u := username(r.cmd.Args[0])
	key, e := queryUserKey(h.db, u)
	if e != nil {
		if e != ErrorDoesNotExist {
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
		}
		return nil, e
	}

	p, err := queryUserPerms(h.db, u)
	if err != nil {
		p = USER // Set to default value
	}

	ret := &User{
		conn:   r.cl,
		name:   u,
		perms:  p,
		pubkey: key,
		secure: r.tls,
	}
	return ret, nil
}

// Check if the user is already logged in from the cache
func (h *Hub) cachedLogin(r Request) (*User, error) {
	id := r.cmd.HD.Op

	v, ok := h.users.Get(r.cl)
	if ok {
		// User is cached and the session can be returned
		return v, nil
	}

	if id == gc.LOGIN {
		// We check if the user is logged in from another IP
		_, ipok := h.findUser(username(r.cmd.Args[0]))
		if ipok {
			// Cannot have two sessions of the same user
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
			return nil, ErrorDuplicatedSession

		}
	}

	// Otherwise the user is not found
	return nil, ErrorDoesNotExist
}

/* HUB QUERY FUNCTIONS */

// Lists all users in the server
func (h *Hub) userlist(online bool) string {
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
		ret, err = queryUsernames(h.db)
		if err != nil {
			gclog.DB("userlist", err)
		}
	}

	// Will return "" if nothing is found
	return ret
}

// Returns an online user if it exists (thread-safe)
func (h *Hub) findUser(uname username) (*User, bool) {
	// This function does not use the generic functions
	// Therefore it must use the asocciated mutex
	list := h.users.GetAll()
	for _, v := range list {
		if v.name == uname {
			return v, true
		}
	}

	return nil, false
}

// Checks a reusable token session
func (h *Hub) checkToken(u User, text string) error {
	if !u.secure {
		return gc.ErrorUnescure
	}

	v, ok := h.verifs.Get(u.name)
	if !ok {
		return gc.ErrorNotFound
	}

	if v.pending {
		return gc.ErrorInvalid
	}

	if v.text != text {
		return gc.ErrorHandshake
	}

	return nil
}

/* HUB MAIN */

// Handles generic server functions
func (hub *Hub) Start() {
	defer hub.db.Close()

	for {
		select {
		case <-hub.shtdwn:
			// Disconnect all users
			list := hub.users.GetAll()
			for _, v := range list {
				v.conn.Close()
			}

			// Wait a bit for everything to close
			time.Sleep(5 * time.Second)

			// Perform a server shutdown
			gclog.Notice("inminent server shutdown")
			os.Exit(0)
		case c := <-hub.clean:
			// Remove all mentions of the user in the cache
			hub.cleanupUser(c)
		}
	}
}
