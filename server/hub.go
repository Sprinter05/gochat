package main

import (
	"crypto/rsa"
	"log"
	"net"
	"sync"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// DEFINITIONS

// Has to be used with net.Conn.RemoteAddr().String()
type ip string

// Has to conform to UsernameSize
type username string

// Specifies a logged in user
type User struct {
	conn   net.Conn
	name   username
	pubkey *rsa.PublicKey
}

// Uses a mutex since functions are running concurrently
type Hub struct {
	req   chan Request
	mut   sync.Mutex
	users map[ip]*User
}

// Specifies the functions to run depending on the ID
type actions func(*Hub, *User, gc.Command)

var cmdTable map[gc.ID]actions = map[gc.ID]actions{
	gc.REG: registerUser,
}

// FUNCTIONS

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

// Check which action to perform
func procRequest(r Request, u *User, h *Hub) {
	id := r.cmd.HD.ID

	// Check if the action can be performed
	fun, ok := cmdTable[id]
	if !ok {
		//* Error with action code
		log.Print("Invalid action performed at hub!")
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

	// Call the function
	go fun(h, user, r.cmd)
}

// Function that distributes actions to run
func (hub *Hub) Run() {
	for {
		// Block until a command is received
		select {
		case r := <-hub.req:
			ip := r.cl.RemoteAddr()
			id := r.cmd.HD.ID

			v, err := hub.logged(ip)
			// If its not registered and the command is not
			// register or connect we return an error to client
			if err != nil && (id != gc.CONN && id != gc.REG) {
				ret := gc.ErrorCode(gc.ErrorNoSession)
				pak, e := gc.NewPacket(gc.ERR, ret, nil)
				if e != nil {
					//* Error when creating packet
					log.Print(e)
				} else {
					// User is not logged in
					r.cl.Write(pak)
				}
				return
			}

			// Process the request
			procRequest(r, v, hub)
		}
	}
}
