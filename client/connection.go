package main

import (
	"fmt"
	"log"

	"github.com/Sprinter05/gochat/internal/spec"
)

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
		fmt.Print("gochat() > ")
	}
}

func ConnectionStart(data *ShellData) {

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
	}
}
