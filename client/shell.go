package main

// This package includes the core functionality of the gochat client shell

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/Sprinter05/gochat/client/commands"
)

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func NewShell(data commands.Command) {
	rd := bufio.NewReader(os.Stdin)
	for {
		PrintPrompt(*data.Data)
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
		if strings.ToUpper(op) == "EXIT" {
			return
		}

		// Sets up command data
		var args [][]byte
		args = append(args, bytes.Fields(input)[1:]...)

		// Gets the appropiate command and executes it
		f := commands.FetchClientCmd(op, data)
		if f == nil {
			continue
		}

		cmdReply := f(data, args...)
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

func ShellPrint(text string, outputType commands.OutputType) {
	prefix := ""
	jump := "\n"
	switch outputType {
	case commands.INTERMEDIATE:
		prefix = "[...] "
	case commands.PACKET:
		prefix = "[PACKET] "
	case commands.PROMPT:
		jump = ""
	case commands.ERROR:
		prefix = "[ERROR] "
	case commands.INFO:
		prefix = "[INFO] "
	case commands.RESULT:
		prefix = "[OK] "
	}

	fmt.Printf("%s%s%s", prefix, text, jump)
}
