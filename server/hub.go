package main

import (
	"log"
	"net"
	"os"
	"strings"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* DATA */

// Function mapping table
// We do not use a variable as a map cannot be const
func lookupCommand(op gc.Action) (action, error) {
	lookup := map[gc.Action]action{
		gc.REG:   registerUser,
		gc.CONN:  connectUser,
		gc.VERIF: verifyUser,
		gc.DISCN: disconnectUser,
		gc.DEREG: deregisterUser,
		gc.REQ:   requestUser,
		gc.USRS:  listUsers,
		gc.MSG:   messageUser,
		gc.RECIV: recivMessages,
		gc.ADMIN: adminOperation,
	}

	v, ok := lookup[op]
	if !ok {
		return nil, ErrorDoesNotExist
	}

	return v, nil
}

/* HUB WRAPPER FUNCTIONS */

// This should be ran every time a connection ends
// Doing so prevents leaking goroutines
func (h *Hub) cleanupConn(cl net.Conn) {
	v, ok := h.runners.Get(cl)
	if !ok {
		// Nothing to cleanup
		return
	}
	close(v)

	// Remove the channel from the map
	// Otherwise a function could send to a closed channel, which would panic
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

	fun, err := lookupCommand(id)
	if err != nil {
		// Invalid action is trying to be ran
		log.Printf("No function asocciated to %s, skipping request!\n", gc.CodeToString(id))
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return
	}

	send, exist := h.runners.Get(u.conn)
	if !exist {
		// Error because the channel does not exist
		//! This should not happen unless the thread panicked
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorUndefined, r.cl)
		log.Fatalf("Cannot send task to %s due to missing channel!", u.name)
		return
	}

	// Send task to the runner
	send <- Task{
		fun:  fun,
		hub:  h,
		user: *u,
		cmd:  r.cmd,
	}
}

// Check if a session is present using the auxiliary functions
func (hub *Hub) checkSession(r Request) (*User, error) {
	cached, err := hub.cachedLogin(r)
	if err == nil {
		// Valid user found in cache, serve request
		return cached, nil
	} else if err != ErrorDoesNotExist {
		// We do not search in the DB if its a different error
		return nil, err
	}

	user, e := hub.dbLogin(r)
	if e == nil {
		// User found in database so we return it
		return user, nil
	} else if e != ErrorDoesNotExist {
		// We do not create a new user if its a different error
		return nil, e
	}

	// Create a new user only if that is what was requested (REG)
	if r.cmd.HD.Op != gc.REG {
		// Cannot do anything else without an account
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorNoSession, r.cl)
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
// Also makes sure that the operation is a handshake operation (CONN or VERIF)
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

	// Check that the operation is correct
	// Must be done after querying the database
	id := r.cmd.HD.Op
	if id != gc.CONN && id != gc.VERIF {
		// If the user is being read from the DB its in handshake
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return nil, ErrorProhibitedOperation
	}

	// We do not need to check the error
	// The part where we check the key already does
	p, err := queryUserPerms(h.db, u)
	if err != nil {
		p = USER // Set to default value
	}

	ret := &User{
		conn:   r.cl,
		name:   u,
		perms:  p,
		pubkey: key,
	}
	return ret, nil
}

// Check if the user is already logged in from the cache
// Also makes sure that the operation is not handshake (REG or CONN)
func (h *Hub) cachedLogin(r Request) (*User, error) {
	id := r.cmd.HD.Op

	// Check if its already IP cached
	v, ok := h.users.Get(r.cl)
	if ok {
		// Operation check must be done after checking the cache
		if id == gc.REG || id == gc.CONN {
			// Can only register or connect if not in cache
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorNoSession, r.cl)
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
		list := h.users.GetAll()
		for _, v := range list {
			str.WriteString(string(v.name) + "\n")
		}

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

	// Will return "" if nothing is found
	return ret
}

// Returns an online user if it exists (thread-safe)
// This function does not use the generic functions
// Therefore it must use the asocciated mutex
func (h *Hub) findUser(uname username) (*User, bool) {
	// Try to find the user
	h.users.mut.RLock()
	defer h.users.mut.RUnlock()
	for _, v := range h.users.tab {
		if v.name == uname {
			return v, true
		}
	}

	return nil, false
}

/* HUB MAIN */

// Function that distributes actions to run
func (hub *Hub) Start() {
	defer hub.db.Close()

	// Does not handle channels being closed
	// Channels used here SHOULDNT be closed
	for {
		select {
		case r, ok := <-hub.req:
			if !ok {
				// Perform a server shutdown
				log.Printf("Shutting server down...\n")
				os.Exit(0)
			}

			// Print command info
			r.cmd.Print()

			// Check if the user can be served
			u, err := hub.checkSession(r)
			if err != nil {
				ip := r.cl.RemoteAddr().String()
				log.Printf("Error checking session from %s: %s\n", ip, err)
				continue // Next request
			}

			hub.procRequest(r, u)
		case c := <-hub.clean:
			// Remove all mentions of the user in the cache
			hub.cleanupUser(c)

			// Close runner for that connection
			hub.cleanupConn(c)
		}
	}
}
