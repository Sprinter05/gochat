package main

// Contains the core functions of the client shell.
// The shell allows the client to send packets to the server.

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Sprinter05/gochat/gcspec"
)

// Interface for all commands
type Command interface {
	Run(id gcspec.ID, inf byte, args []string, nArg int) error
}

// Type for commands with arguments
type CmdArgs func(id gcspec.ID, inf byte, args []string, nArg int) error

func (cmd CmdArgs) Run(id gcspec.ID, inf byte, args []string, nArg int) error {
	return cmd(id, inf, args, nArg)
}

// Type for commands with no arguments
type CmdNoArgs func() error

func (cmd CmdNoArgs) Run(id gcspec.ID, inf byte, args []string, nArg int) error {
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
	"USRS":  1,
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
		input, err := rd.ReadString('\n')
		if err != nil {
			fmt.Println(err)
		}
		// Clears any leading an trailing spaces along with the newline character
		input = strings.TrimSpace(input)
		// Splits the command and arguments
		cmd := strings.Fields(input)[0]
		args := strings.Fields(input)[1:]

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
			err := v.Run(gcspec.ID(gcspec.StringToCode(cmd)), gcspec.EmptyInfo, args, nArgs[cmd])
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
	// Attempts to open the file and gets its content
	txt, err := os.ReadFile("../doc/HELP.txt")
	if err != nil {
		return fmt.Errorf("HELP: Unable to open HELP.txt")
	}
	fmt.Println(string(txt))
	return nil
}

// Generic function able to execute every packet-sending command
func sendPacket(id gcspec.ID, inf byte, args []string, nArg int) error {
	// Checks argument count
	if len(args) != nArg {
		return fmt.Errorf("%s: Incorrect number of arguments", gcspec.CodeToString(id))
	}
	// Creates packet with the proper headers
	pct, err := gcspec.NewPacket(id, gcspec.EmptyInfo, args)
	if err != nil {
		return fmt.Errorf("%s: %s", gcspec.CodeToString(id), err)
	}
	// Sends packet to server
	_, err = gCon.Write(pct)
	if err != nil {
		return fmt.Errorf("%s: Unable to write packet to connection", gcspec.CodeToString(id))
	}

	return nil
}
