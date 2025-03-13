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

// Interface for all commands
type Command interface {
	Run(id gcspec.Action, act gcspec.ID, inf byte, args []gcspec.Arg, nArg int) error
}

// Type for commands with arguments
type CmdArgs func(act gcspec.Action, id gcspec.ID, inf byte, args []gcspec.Arg, nArg int) error

func (cmd CmdArgs) Run(act gcspec.Action, id gcspec.ID, inf byte, args []gcspec.Arg, nArg int) error {
	return cmd(act, id, inf, args, nArg)
}

// Type for commands with no arguments
type CmdNoArgs func() error

func (cmd CmdNoArgs) Run(act gcspec.Action, id gcspec.ID, inf byte, args []gcspec.Arg, nArg int) error {
	return cmd()
}

// Map with all comands except EXIT
var cmds = map[string]Command{
	"VER":   CmdNoArgs(ver),
	"HELP":  CmdNoArgs(help),
	"REG":   CmdArgs(sendPacket),
	"CONN":  CmdArgs(sendPacket),
	"VERIF": CmdArgs(sendPacket),
	"REQ":   CmdArgs(sendPacket),
	"USRS":  CmdArgs(sendPacket),
	"MSG":   CmdArgs(sendPacket),
	"DISCN": CmdArgs(sendPacket),
	"DEREG": CmdArgs(sendPacket),
}

// Map that associates the number of arguments required for each command
var nArgs = map[string]int{
	"VER":   0,
	"HELP":  0,
	"REG":   2,
	"CONN":  1,
	"VERIF": 1,
	"REQ":   1,
	"USRS":  0,
	"MSG":   3,
	"DISCN": 0,
	"DEREG": 0,
}

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
func ver() error {
	fmt.Printf("gochat version %d\n", gcspec.ProtocolVersion)
	return nil
}

// Execution code of the HELP command
func help() error {
	fmt.Println(helpText)
	return nil
}

// Generic function able to execute every packet-sending command
func sendPacket(act gcspec.Action, id gcspec.ID, inf byte, args []gcspec.Arg, nArg int) error {
	// Checks argument count
	if len(args) != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", gcspec.CodeToString(act))
	}
	// Creates packet with the proper headers
	pct, err := gcspec.NewPacket(act, id, gcspec.EmptyInfo, args)
	if err != nil {
		return fmt.Errorf("%s: %s", gcspec.CodeToString(act), err)
	}
	// Sends packet to server
	_, errW := gCon.Write(pct)
	if errW != nil {
		return fmt.Errorf("%s: Unable to write packet to connection", gcspec.CodeToString(act))
	} else {
		// If the packet is sent correctly, it is added to PacketBuffer
		PacketBuffer[uint16(id)] = &pct
	}

	return nil
}
