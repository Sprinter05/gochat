package main

// This package includes the core functionality of the gochat client shell

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func NewShell(con net.Conn, verbose *bool) {
	rd := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("gochat() > ")
		// Reads user input
		input, readErr := rd.ReadBytes('\n')
		if readErr != nil {
			fmt.Printf("input error: %s\n", readErr)
			continue
		}
		// Trims the input, removing trailing spaces and line jumps
		input = bytes.TrimSpace(input)
		if len(input) == 0 {
			// Empty command, asks for input again
			continue
		}

		op := string(bytes.Fields(input)[0])
		if op == "EXIT" {
			return
		}

		// Sets up command data
		var args [][]byte
		args = append(args, bytes.Fields(input)[1:]...)
		data := CommandData{Args: args, Con: con}

		// Gets the appropiate command and executes it
		f := FetchClientCmd(op)
		if f == nil {
			continue
		}

		err := f(data, verbose)
		if err != nil {
			fmt.Printf("%s: %s\n", op, err)
		}

		// TODO: NewPacket will be moved to each command function to fit the needs of each operation once they are ready
		pct, pctErr := spec.NewPacket(spec.Action(spec.StringToCode(op)), 1, spec.EmptyInfo, args...) // TODO: ID assignation
		if pctErr != nil {
			fmt.Printf("packet creation error: %s\n", pctErr)
			continue
		}

		if *verbose {
			cmd := spec.ParsePacket(pct)
			if cmd.HD.Op != 0 {
				cmd.Print()
			}
		}
	}
}
