package main

import "fmt"

type Hub struct {
	comm  chan Client
	users map[string]Client
}

func (hub *Hub) Run() {
	for {
		// Block until a command is received
		select {
		case cmd := <-hub.comm:
			fmt.Println(cmd)
		}
	}
}
