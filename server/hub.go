package main

import (
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* DATA */

// Function mapping table
var cmdTable map[gc.Action]actions = map[gc.Action]actions{
	gc.REG: registerUser,
}

/* AUXILIARY FUNCTIONS */

// Check which action to perform
func procRequest(r Request, u *User, h *Hub) {
	id := r.cmd.HD.Op

	// Check if the action can be performed
	fun, ok := cmdTable[id]
	if !ok {
		//* Error with action code
		log.Println("Invalid action performed at hub!")
		return
	}

	//! Be careful with race condition
	//TODO: Maybe lock to only 1 action per user?
	go fun(h, u, r.cmd)
}

/* HUB FUNCTIONS */

// Check if there is a possible login from the database
func (h *Hub) dbLogin(r Request) (*User, error) {
	ip := ip(r.cl.RemoteAddr().String())

	// Check if the user is in the database
	u := username(r.cmd.Args[0])
	key, e := queryUserKey(h.db, u)
	if e == nil {
		// User is in the database so we query it
		u := &User{
			conn:   r.cl,
			name:   u,
			pubkey: key,
		}

		// Cache user from now on
		h.mut.Lock()
		h.users[ip] = u
		h.mut.Unlock()

		// Return user
		return u, nil
	}

	return nil, gc.ErrorNotFound
}

// Check if the user is already logged in from the cache
func (h *Hub) cachedLogin(r Request) (*User, error) {
	ip := r.cl.RemoteAddr()
	id := r.cmd.HD.Op

	// Check if its already IP cached
	v, err := h.loggedIP(ip)
	if err == nil {
		if id == gc.REG || id == gc.CONN {
			// If its logged in and the command is REG OR CONN we error
			sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
			return nil, err
		} else {
			// User is cached and the request can be served
			return v, nil
		}
	}

	// We check if the user is logged in from another IP
	if h.userLogged(username(r.cmd.Args[0])) {
		// Cannot have two sessions of the same user
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorLogin, r.cl)
		return nil, gc.ErrorLogin
	}

	// Otherwise we return the value
	return nil, gc.ErrorNotFound
}

// Find a username in case it might be logged in with a different IP
func (hub *Hub) userLogged(uname username) bool {
	hub.mut.Lock()
	defer hub.mut.Unlock()
	for _, v := range hub.users {
		if v.name == uname {
			return true
		}
	}

	// User is not found
	return false
}

// Check if a user is already loggedIP in
func (hub *Hub) loggedIP(addr net.Addr) (*User, error) {
	ip := ip(addr.String())

	// Check if IP is already cached
	hub.mut.Lock()
	v, ok := hub.users[ip]
	hub.mut.Unlock()

	if ok {
		return v, nil
	}

	return nil, gc.ErrorNotFound
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
	if err != nil {
		return user, nil
	}

	// Fallthrough
	return nil, nil
}

// Function that distributes actions to run
func (hub *Hub) Run() {
	defer hub.db.Close()

	// Read the channel until closed
	for r := range hub.req {
		// Print command info
		r.cmd.Print()

		// Check if the user can be served
		u, e := hub.checkSession(r)
		if e != nil {
			log.Println(e)
			continue
		}

		// TODO: Create new user if not in db
		// TODO: CONN protection?

		// Process the request
		procRequest(r, u, hub)
	}
}
