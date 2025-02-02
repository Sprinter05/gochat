package main

import gc "github.com/Sprinter05/gochat/gcspec"

type Hub struct {
	comm  chan Request
	users map[string]gc.Connection
}

func (hub *Hub) Run() {
	for {
		// Block until a command is received
		select {
		case c := <-hub.comm:
			c.cmd.Print()
		}
	}
}
