package main

// This package includes the core functionality of the gochat client shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/Sprinter05/gochat/client/commands"
)

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func NewShell(data *commands.Data) {
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
		f := commands.FetchClientCmd(op, ShellPrint)
		if f == nil {
			continue
		}

		cmdReply := f(data, ShellPrint, args...)
		if cmdReply.Error != nil {
			fmt.Printf("%s: %s\n", op, cmdReply.Error)
		}
	}
}

func PrintPrompt(data commands.Data) {
	connected := ""
	username := ""
	if !(data.User.User.Username == "") {
		username = data.User.User.Username
	}
	if data.ClientCon.Conn == nil {
		connected = "(not connected) "
	}
	fmt.Printf("\033[36m%sgochat(%s) > \033[0m", connected, username)
}

func ShellPrint(text string) {
	fmt.Print(text)
}
