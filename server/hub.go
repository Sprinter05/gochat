package main

import (
	"log"
	"net"
	"strings"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* DATA */

// Function mapping table
// ? Dangerous global variable
var cmdTable map[gc.Action]actions = map[gc.Action]actions{
	gc.REG:   registerUser,
	gc.CONN:  connectUser,
	gc.VERIF: verifyUser,
	gc.DISCN: disconnectUser,
	gc.DEREG: deregisterUser,
	gc.REQ:   requestUser,
	gc.USRS:  listUsers,
}

/* HUB WRAPPER FUNCTIONS */

// Lists all users in the server
func (h *Hub) userlist(online bool) string {
	var str strings.Builder
	var ret string

	if online {
		ret = ""
		for _, v := range h.users {
			str.WriteString(string(v.name) + "\n")
		}

		l := str.Len()
		ret = str.String()

		// Remove the last newline
		ret = ret[:l-1]
	} else {
		// Query database
		ret, _ = queryUsernames(h.db)
	}

	// Will return empty if nothing is found
	return ret
}

// Cleans any mention to a connection in the caches
func (h *Hub) cleanupConn(cl net.Conn) {
	// Cleanup on the users table
	h.umut.Lock()
	_, uok := h.users[cl]
	if uok {
		delete(h.users, cl)
	}
	h.umut.Unlock()

	// Cleanup on the verification table
	h.vmut.Lock()
	_, vok := h.verifs[cl]
	if vok {
		delete(h.verifs, cl)
	}
	h.vmut.Unlock()
}

// Check which action to perform
func (h *Hub) procRequest(r Request, u *User) {
	id := r.cmd.HD.Op

	// Check if the action can be performed
	fun, ok := cmdTable[id]
	if !ok {
		log.Println("Invalid action performed at hub!")
		return
	}

	// If the user is null we create a new one
	var user *User
	if u == nil {
		user = &User{
			conn: r.cl,
		}
	} else {
		user = u
	}

	// TODO: Add "runners" per client that just run the request
	fun(h, user, r.cmd)
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) checkSession(r Request) (*User, error) {
	// Check the user session
	user, err := hub.cachedLogin(r)
	if err == nil {
		// Valid user found in cache, serve request
		return user, nil
	} else if err != gc.ErrorNotFound {
		// We do not search in the DB if its a different error
		return nil, err
	}

	// Query the database
	user, err = hub.dbLogin(r)
	if err == nil {
		return user, nil
	}

	// Fallthrough
	return nil, nil
}

/* HUB LOGIN FUNCTIONS */

// Check if there is a possible login from the database
func (h *Hub) dbLogin(r Request) (*User, error) {
	// Check that the operation is correct before querying the database
	id := r.cmd.HD.Op
	if id != gc.CONN && id != gc.VERIF {
		//* If the user is being read from the DB its in handshake
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return nil, gc.ErrorInvalid
	}

	// Check if the user is in the database
	u := username(r.cmd.Args[0])
	key, e := queryUserKey(h.db, u)
	if e == nil {
		// If the key is null the user has been deregisterd
		if key == nil {
			return nil, gc.ErrorLogin
		}

		// User is in the database so we query it
		u := &User{
			conn:   r.cl,
			name:   u,
			pubkey: key,
		}

		// Return user
		return u, nil
	}

	return nil, gc.ErrorNotFound
}

// Check if the user is already logged in from the cache
func (h *Hub) cachedLogin(r Request) (*User, error) {
	id := r.cmd.HD.Op

	// Check if its already IP cached
	v, err := h.loggedConn(r.cl)
	if err == nil {
		if id == gc.REG || id == gc.CONN {
			//* Can only register or connect if not in cache
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
			return nil, gc.ErrorInvalid
		} else {
			// User is cached and the session can be returned
			return v, nil
		}
	}

	// We check if the user is logged in from another IP
	if h.userLogged(username(r.cmd.Args[0])) {
		//* Cannot have two sessions of the same user
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
		return nil, gc.ErrorLogin
	}

	// Otherwise we return the value
	return nil, gc.ErrorNotFound
}

/* HUB CHECK FUNCTIONS */

// Find a username in case it might be logged in with a different IP
func (hub *Hub) userLogged(uname username) bool {
	hub.umut.Lock()
	defer hub.umut.Unlock()
	for _, v := range hub.users {
		if v.name == uname {
			return true
		}
	}

	// User is not found
	return false
}

// Check if a user is already loggedConn in
func (hub *Hub) loggedConn(conn net.Conn) (*User, error) {
	// Check if IP is already cached
	hub.umut.Lock()
	v, ok := hub.users[conn]
	hub.umut.Unlock()

	if ok {
		return v, nil
	}

	return nil, gc.ErrorNotFound
}

// Function that distributes actions to run
func (hub *Hub) Run() {
	// Close database at exit
	defer hub.db.Close()

	// Read the channel until closed
	for {
		select {
		case r := <-hub.req:
			// Print command info
			r.cmd.Print()

			// Check if the user can be served
			u, e := hub.checkSession(r)
			if e != nil {
				log.Println(e)
				continue
			}

			// Process the request
			hub.procRequest(r, u)
		case c := <-hub.clean:
			// Remove all mentions of the connection in the cache
			hub.cleanupConn(c)
		}
	}

	// TODO: Add shutdown function for all clients
	//time.Now().Unix()
}
