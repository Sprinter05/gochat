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
var cmdTable map[gc.Action]action = map[gc.Action]action{
	gc.REG:   registerUser,
	gc.CONN:  connectUser,
	gc.VERIF: verifyUser,
	gc.DISCN: disconnectUser,
	gc.DEREG: deregisterUser,
	gc.REQ:   requestUser,
	gc.USRS:  listUsers,
	gc.MSG:   messageUser,
	gc.RECIV: recivMessages,
}

/* HUB WRAPPER FUNCTIONS */

// This should be run every time a connection ends
// Doing so prevents leaking goroutines
func (h *Hub) cleanupConn(cl net.Conn) {
	// Close the channel to stop the goroutine
	v, ok := h.runners.Get(cl)
	if !ok {
		// Nothing to cleanup
		return
	}
	close(v)

	// Remove the channel from the map
	h.runners.Remove(cl)
}

// Cleans any mention to a connection in the caches
func (h *Hub) cleanupUser(cl net.Conn) {
	// Cleanup on the users table
	h.users.Remove(cl)

	// Cleanup on the verification table
	h.verifs.Remove(cl)

	// Remove runner from the table
	h.runners.Remove(cl)
}

// Check which action to perform
func (h *Hub) procRequest(r Request, u *User) {
	id := r.cmd.HD.Op

	// Check if the action can be performed
	fun, ok := cmdTable[id]
	if !ok {
		// Invalid action is trying to be ran
		log.Printf("No function asocciated to %s, ignoring request!\n", gc.CodeToString(id))
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return
	}

	// Check if the channel exists and create it otherwise
	send, exist := h.runners.Get(u.conn)
	if !exist {
		// Error because the channel does not exist
		log.Printf("Cannot send task to %s due to missing channel", u.name)
	}

	// Send task to the channel
	send <- Task{
		fun:  fun,
		hub:  h,
		user: u,
		cmd:  r.cmd,
	}
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) checkSession(r Request) (*User, error) {
	// Check the user session
	cached, err := hub.cachedLogin(r)
	if err == nil {
		// Valid user found in cache, serve request
		return cached, nil
	} else if err != ErrorDoesNotExist {
		// We do not search in the DB if its a different error
		return nil, err
	}

	// Query the database
	user, e := hub.dbLogin(r)
	if e == nil {
		// User found in database so we return it
		return user, nil
	} else if e != ErrorDoesNotExist {
		// We do not create a new user if its a different error
		return nil, e
	}

	// Create a new user only if that is what was requested
	if r.cmd.HD.Op != gc.REG {
		// Cannot do anything else without an account
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return nil, ErrorNoAccount
	}

	// Newly created user
	return &User{
		conn: r.cl,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Check if there is a possible login from the database
// Also makes sure that the operation is a handshake operation
func (h *Hub) dbLogin(r Request) (*User, error) {
	// Check that the operation is correct before querying the database
	id := r.cmd.HD.Op
	if id != gc.CONN && id != gc.VERIF {
		// If the user is being read from the DB its in handshake
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return nil, ErrorProhibitedOperation
	}

	// Check if the user is in the database
	u := username(r.cmd.Args[0])
	key, e := queryUserKey(h.db, u)
	if e != nil {
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
		return nil, e
	}

	// User is in the database so we query it
	ret := &User{
		conn:   r.cl,
		name:   u,
		pubkey: key,
	}

	// Return user
	return ret, nil
}

// Check if the user is already logged in from the cache
// Also makes sure that the operation is not trying to register or connect
func (h *Hub) cachedLogin(r Request) (*User, error) {
	id := r.cmd.HD.Op

	// Check if its already IP cached
	v, ok := h.users.Get(r.cl)
	if ok {
		if id == gc.REG || id == gc.CONN {
			// Can only register or connect if not in cache
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
			return nil, ErrorSessionExists
		} else {
			// User is cached and the session can be returned
			return v, nil
		}
	}

	// We check if the user is logged in from another IP
	if _, ok := h.findUser(username(r.cmd.Args[0])); ok {
		// Cannot have two sessions of the same user
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
		return nil, ErrorDuplicatedSession
	}

	// Otherwise we return the value
	return nil, ErrorDoesNotExist
}

/* HUB QUERY FUNCTIONS */

// Lists all users in the server
func (h *Hub) userlist(online bool) string {
	var str strings.Builder
	var ret string = ""
	var err error

	if online {
		h.users.mut.Lock()
		for _, v := range h.users.tab {
			str.WriteString(string(v.name) + "\n")
		}
		h.users.mut.Unlock()

		l := str.Len()
		ret = str.String()

		// Remove the last newline
		ret = ret[:l-1]
	} else {
		// Query database
		ret, err = queryUsernames(h.db)
		if err != nil {
			log.Printf("Error querying username list: %s\n", err)
		}
	}

	// Will return empty if nothing is found
	return ret
}

// Returns an online user if it exists
func (h *Hub) findUser(uname username) (*User, bool) {
	// Try to find the user
	h.users.mut.Lock()
	for _, v := range h.users.tab {
		if v.name == uname {
			return v, true
		}
	}
	h.users.mut.Unlock()

	return nil, false
}

/* HUB MAIN */

// Function that distributes actions to run
func (hub *Hub) Start() {
	// Close database at exit
	defer hub.db.Close()

	// Not prepared for channels being closed
	// Channels shouldnt be able to be closed
	for {
		select {
		case r := <-hub.req:
			// Print command info
			r.cmd.Print()

			// Check if the user can be served
			u, err := hub.checkSession(r)
			if err != nil {
				ip := r.cl.RemoteAddr().String()
				log.Printf("Error checking session from %s: %s\n", ip, err)
				continue // Next request
			}

			// Process the request
			hub.procRequest(r, u)
		case c := <-hub.clean:
			// Remove all mentions of the connection in the cache
			hub.cleanupUser(c)

			// Close all runners
			hub.cleanupConn(c)
		}
	}

	// TODO: Add shutdown function for all clients
}
