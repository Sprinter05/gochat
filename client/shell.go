package main

// This package includes the core functionality of the gochat client shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
)

// Struct that contains all the data required for the shell to function.
// Commands may alter the data if necessary
type ShellData struct {
	ClientCon spec.Connection
	Verbose   bool
	DB        *gorm.DB
	Server    Server
	// TODO: Logged in user
}

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func NewShell(data *ShellData) {
	rd := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\033[36mgochat() > \033[0m")
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

		// Gets the appropiate command and executes it
		f := FetchClientCmd(op)
		if f == nil {
			continue
		}

		err := f(data, args)
		if err != nil {
			fmt.Printf("%s: %s\n", op, err)
		}
	}
}
