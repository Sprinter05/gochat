package main

import (
	"fmt"
	"log"
	"slices"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Listens for incoming server packets. It also executes
// the appropiate client actions depending on the packet received
func Listen(data *ShellData) {
	defer data.ClientCon.Conn.Close()

	for {
		cmd := spec.Command{}

		// Header listen
		hdErr := cmd.ListenHeader(data.ClientCon)
		if hdErr != nil {
			continue
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			fmt.Print("\r\033[K")
			fmt.Println("invalid header received from server:")
			if data.Verbose {
				cmd.Print()
			}
			fmt.Print("gochat() > ")
			continue
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.ClientCon)
		if pldErr != nil {
			continue
		}

		fmt.Print("\r\033[K")
		fmt.Println("Packet received from server:")
		if data.Verbose {
			cmd.Print()
		}
		PrintPrompt(*data)
	}
}

// Listens for an OK packet from the server when starting the connection,
// which determines that the client/server was started successfully
func ConnectionStart(data ShellData) {

	cmd := spec.Command{}

	// Header listen
	hdErr := cmd.ListenHeader(data.ClientCon)
	if hdErr != nil {
		log.Fatal("could not connect to server: invalid header received")
	}

	// Header check
	chErr := cmd.HD.ClientCheck()
	if chErr != nil {
		if data.Verbose {
			cmd.Print()
		}
		log.Fatal("could not connect to server: malformed header received")
	}

	if cmd.HD.Op == 1 {
		fmt.Println("successfully connected to the server")
	} else {
		log.Fatal("could not connect to server: unexpected action code received")
	}
}

// Receives a slice of command operations to listen to, then starts
// listening until a received packet fits one of the actions provided
// and returns it
func ListenResponse(data ShellData, id spec.ID, ops ...spec.Action) (spec.Command, error) {
	// TODO: timeouts
	var cmd spec.Command

	for !(slices.Contains(ops, cmd.HD.Op)) {
		cmd = spec.Command{}
		// Header listen
		hdErr := cmd.ListenHeader(data.ClientCon)
		if hdErr != nil {
			return cmd, hdErr
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			if data.Verbose {
				cmd.Print()
			}
			return cmd, chErr
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.ClientCon)
		if pldErr != nil {
			return cmd, pldErr
		}
	}

	if data.Verbose {
		fmt.Println("Packet received from server:")
		cmd.Print()
	}

	if cmd.HD.ID != id {
		return cmd, fmt.Errorf("unexpected ID received")
	}
	return cmd, nil
}
