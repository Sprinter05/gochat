package main

import (
	"crypto/rsa"
	"sync"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// Has to be used with net.Conn.RemoteAddr().String()
type ip string

// Has to conform to UsernameSize
type username string

type User struct {
	conn   *gc.Connection
	name   username
	pubkey *rsa.PublicKey
}

// conns will be mapped to users
type Hub struct {
	req   chan Request
	mut   sync.Mutex
	users map[username]*User
	conns map[ip]username
}

func (hub *Hub) Run() {
	for {
		// Block until a command is received
		select {
		case c := <-hub.req:
			c.cmd.Print()

		}
	}
}
