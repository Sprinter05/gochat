package main

import (
	"fmt"
	"net"

	"github.com/Sprinter05/gochat/internal/spec"
)

func Listen(con net.Conn, verbose *bool) {
	cl := spec.Connection{
		Conn: con,
	}
	defer cl.Conn.Close()

	for {
		cmd := spec.Command{}

		// Header listen
		hdErr := cmd.ListenHeader(cl)
		if hdErr != nil {
			fmt.Printf("error in header listen: %s\n", hdErr)
			continue
		}

		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			fmt.Printf("malformed header received: %s\n", chErr)
			continue
		}

		// Payload listen
		pldErr := cmd.ListenPayload(cl)
		if pldErr != nil {
			fmt.Printf("error in payload listen: %s\n", pldErr)
			continue
		}

		fmt.Println("Packet received from server")
		if *verbose {
			cmd.Print()
		}
	}
}
