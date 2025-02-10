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
	gc.MSG:   messageUser,
}

/* HUB WRAPPER FUNCTIONS */

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

// Perform a catch up for a user
func (h *Hub) wrapCatchUp(u *User) {
	// Get the amount of messages needed
	size, err := queryMessageQuantity(h.db, u.name)
	if err != nil {
		log.Printf("Could not query message quantity for %s: %s\n", u.name, err)
	}
	if size == 0 {
		// Nothing to do
		return
	}

	catch, err := queryMessages(h.db, u.name, size)
	if err != nil {
		log.Printf("Could not query messages for %s: %s\n", u.name, err)
	}

	// Do the catch up concurrently
	go catchUp(u.conn, catch)

	// Get the timestamp of the newest message as threshold
	ts := (*catch)[size].stamp
	removeMessages(h.db, u.name, ts)
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

	// TODO: Add "runners" per client that just run the request
	fun(h, u, r.cmd)
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
	v, ok := h.loggedConn(r.cl)
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
	if h.userLogged(username(r.cmd.Args[0])) {
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
		h.umut.Lock()
		for _, v := range h.users {
			str.WriteString(string(v.name) + "\n")
		}
		h.umut.Unlock()

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
func (hub *Hub) loggedConn(conn net.Conn) (*User, bool) {
	// Check if IP is already cached
	hub.umut.Lock()
	v, ok := hub.users[conn]
	hub.umut.Unlock()

	if ok {
		return v, true
	}

	// User is not in the cache
	return nil, false
}

// Returns an online user if it exists
func (h *Hub) findUser(uname username) (*User, bool) {
	// Try to find the user
	h.umut.Lock()
	for _, v := range h.users {
		if v.name == uname {
			return v, true
		}
	}
	h.umut.Unlock()

	return nil, false
}

/* HUB MAIN */

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
			hub.cleanupConn(c)
		}
	}

	// TODO: Add shutdown function for all clients
	//time.Now().Unix()
}
