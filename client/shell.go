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
type Data struct {
	ClientCon spec.Connection
	Verbose   bool
	ShellMode bool // If ShellMode is true, the struct belongs to the shell and the output should be printed
	DB        *gorm.DB
	Server    Server
	User      LocalUserData
}

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func NewShell(data *Data) {
	rd := bufio.NewReader(os.Stdin)
	for {
		PrintPrompt(*data)
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

		cmdReply := f(data, args)
		if cmdReply.Error != nil {
			fmt.Printf("%s: %s\n", op, cmdReply.Error)
		}
	}
}

func PrintPrompt(data Data) {
	username := ""
	if !(data.User.User.Username == "") {
		username = data.User.User.Username
	}
	fmt.Printf("\033[36mgochat(%s) > \033[0m", username)
}
