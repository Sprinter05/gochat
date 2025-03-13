package main

// Contains the core functions of the client shell.
// The shell allows the client to send packets to the server.

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"

	"github.com/Sprinter05/gochat/gcspec"
)

// Text to be printed by the HELP command
const helpText = "EXIT: Closes the shell.\n\n" +

	"VER: Prints out the gochat version the client has installed.\n\n" +

	"VERBOSE: Enables/disables verbose mode.\n\n" +

	"HELP: Prints out a manual for the use of this shell.\n\n" +

	"VERBOSE: Toggles on/off the verbose mode\n\n" +

	"CREATEUSER <username>: Creates a user and adds it to the client database\n\n" +

	"REG <rsa_pub> <username>: Provides the generated RSA public key and username to register to the server.\n\n" +

	"REGUSER: Sends a REG packet to the server to register the current shell user\n\n" +

	"LOGIN <username>: Connects to the server by providing the already generated RSA public key.\n\n" +

	"VERIF <decyphered_text>: Replies to the server's verification request, providing the decyphered_text.\n\n" +

	"REQ <username>: Used to request a connection with another client in order to begin messaging.\n\n" +

	"USRS <online/all>: Requests the server a list of either the users online or all of them, depending on the token specified on the argument.\n\n" +

	"MSG <username> <unix_stamp> <cypher_payload>: Sends a message to the specified user, providing the specified UNIX timestamp and the payload, which is the chyphered text message.\n\n" +

	"RECIV: Sends a catch-up request to the server\n\n" +

	"LOGOUT: Disconnects the client from the server.\n\n" +

	"DEREG: Deregisters the user from the server."

// Variable that defines whether the shell should be verbose or not
// If the shell is verbose, it will print each packet that is received
// whether it should be printed or not.
var IsVerbose bool = false

// Type for commands with arguments
// Globalizes the connection variable obtained by the NewShell argument
var gCon net.Conn

// Initializes a client shell
func NewShell(con net.Conn) {

	gCon = con
	rd := bufio.NewReader(os.Stdin)

	// Runs inconditionally until EXIT is executed
	for {
		PrintPrompt()
		// Starts reading input
		input, err := rd.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
		}
		// Clears any leading an trailing spaces along with the newline character
		input = bytes.TrimSpace(input)
		// Splits the command and arguments
		cmd := string(bytes.Fields(input)[0])

		// Casts every arg byte array into Arg type to append it to the argument slice
		var args []gcspec.Arg
		for _, arg := range bytes.Fields(input)[1:] {
			args = append(args, gcspec.Arg(arg))
		}

		if cmd == "EXIT" {
			// Closes the shell
			return
		}

		// Checks if the command exists in order to execute it
		v, ok := cmds[cmd]
		if !ok {
			fmt.Printf("%s: No such command\n", cmd)
		} else {
			// Runs command
			err := v.Run(gcspec.Action(gcspec.StringToCode(cmd)), gcspec.ID(gcspec.GeneratePacketID(PacketBuffer)), gcspec.EmptyInfo, args, nArgs[cmd]) // TODO: Change "24"
			if err != nil {
				fmt.Println(err)
			}
		}
	}
}

// Prints an ANSI escape code to reset the current line in case a message is received
func ClearPrompt() {
	fmt.Print("\r\033[K")
}

// Prints the shell prompt
func PrintPrompt() {
	fmt.Print("gochat > ")
}

// COMMANDS

// Execution code of the VER command
}
