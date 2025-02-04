package main

import (
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* DATA */

// Function mapping table
var cmdTable map[gc.ID]actions = map[gc.ID]actions{
	gc.REG: registerUser,
}

/* AUXILIARY FUNCTIONS */

// Check which action to perform
func procRequest(r Request, u *User, h *Hub) {
	id := r.cmd.HD.ID

	// Check if the action can be performed
	fun, ok := cmdTable[id]
	if !ok {
		//* Error with action code
		log.Println("Invalid action performed at hub!")
		return
	}

	// If we are going to register we create a new user
	var user *User
	if id == gc.REG {
		user = &User{
			conn: r.cl,
		}
	} else {
		user = u
	}

	//! Be careful with race condition
	//TODO: Maybe lock to only 1 action per user?
	go fun(h, user, r.cmd)
}

func readRequest(r Request, h *Hub) (*User, error) {
	ip := r.cl.RemoteAddr()
	id := r.cmd.HD.ID

	// Check if its already logged in
	v, err := h.logged(ip)

	// If its not registered and the command is not
	// register or connect we return an error to client
	if err != nil && (id != gc.CONN && id != gc.REG) {
		ret := gc.ErrorCode(gc.ErrorNoSession)
		pak, e := gc.NewPacket(gc.ERR, r.cmd.HD.Ord, ret, nil)
		if e != nil {
			//* Error when creating packet
			return nil, e
		} else {
			//* Error since user is not logged in
			r.cl.Write(pak)
			return nil, err
		}
	}

	// Otherwise we return the value
	//! The user returned can be nil but should be handled by procRequest
	return v, nil
}

/* HUB FUNCTIONS */

// Check if a user is already logged in
func (hub *Hub) logged(addr net.Addr) (*User, error) {
	ip := ip(addr.String())

	// Check if user is already cached
	hub.mut.Lock()
	v, ok := hub.users[ip]
	hub.mut.Unlock()

	if ok {
		return v, nil
	}
	return nil, gc.ErrorNotFound

}

// Function that distributes actions to run
func (hub *Hub) Run() {
	for {
		// Block until a command is received
		select {
		case r := <-hub.req:
			// Print command info
			r.cmd.Print()

			// Read the incoming request
			v, err := readRequest(r, hub)
			if err != nil {
				log.Println(err)
				continue
			}

			// Process the request
			procRequest(r, v, hub)
		}
	}
}
