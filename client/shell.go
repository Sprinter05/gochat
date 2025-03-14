package main

// Contains the core functions of the client shell.
// The shell allows the client to send packets to the server.
// ! Mete la shell en un subpaquete para abstraerlo del resto de la logica

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"

	"slices"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Text to be printed by the HELP command
// ! No uses concatenacion de strings
// ! Si haces:
// ! `
// ! Puedes escribir en medio de estos apostrofes
// ! Y ya te añade las lineas nuevas automaticemente
// ! `
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
// ! Variable global mal
var IsVerbose bool = false

// Globalizes the connection variable obtained by the NewShell argument.
// ! Variable global mal
var gCon net.Conn

// Stores the current user using the shell
// ! Variable global mal
var CurUser Client

// Initializes a client shell.
// ! pctReceived deberia ser <- (read only), asi evitas accidentalmente mandar en ese canal
// ! Al pasar canales entre functiones nunca deberia ser bidireccional, si lo es, estas planteando algo mal
func NewShell(con net.Conn, ctx context.Context, pctReceived chan struct{}) {
	gCon = con
	rd := bufio.NewReader(os.Stdin)
	// Runs inconditionally until EXIT is executed
	for {
		ClearPrompt()
		PrintPrompt()
		// Starts reading input
		input, err := rd.ReadBytes('\n')
		if err != nil {
			fmt.Println(err)
			continue
		}
		// Clears any leading an trailing spaces along with the newline character
		input = bytes.TrimSpace(input)
		// Asks for input again if the received input is empty after trimming
		if len(input) == 0 {
			continue
		}

		// Splits the command and arguments
		instruction := string(bytes.Fields(input)[0])

		// Casts every arg byte array into Arg type to append it to the argument slice
		var args [][]byte
		args = append(args, bytes.Fields(input)[1:]...)

		if instruction == "EXIT" {
			return
		}

		v, ok := ClientCmds[instruction]
		if !ok {
			fmt.Printf("%s: No such command\n", instruction)
		} else {
			payloadLen := 0
			for _, arg := range args {
				payloadLen += len(arg) + 2 // + 2 to include the CRLF in each argument
			}
			// ! Usa NewPacket()
			// Creates header
			header := spec.Header{
				Ver:  spec.ProtocolVersion,
				Op:   spec.Action(spec.StringToCode(instruction)),
				Info: spec.EmptyInfo,
				Args: uint8(len(args)),
				Len:  uint16(payloadLen),
				ID:   spec.ID(spec.GeneratePacketID(PendingBuffer)),
			}
			// Creates command
			cmd := spec.Command{HD: header, Args: args}

			// Runs command
			err := v.Run(&cmd, NumArgs[instruction])
			if err != nil {
				fmt.Println(err)
			}

			if requiresSync(instruction) && err == nil {
				select {
				case <-pctReceived:
					fmt.Println("Packet received")
				case <-ctx.Done():
					fmt.Println("Timeout")
				}
			}
		}
	}
}

// Prints an ANSI escape code to reset the current line in case a message is received.
func ClearPrompt() {
	fmt.Print("\r\033[K")
}

func PrintPrompt() {
	fmt.Print("gochat > ")
}

func requiresSync(instruction string) bool {
	// ! Este slice se esta redeclarando cada vez que llamas a la funcion, considera hacerlo de otra forma
	syncCommands := []string{"REGUSER", "REG", "LOGIN", "VERIF", "REQ", "USRS", "MSG", "RECIV", "DISCN", "DEREG"}
	return slices.Contains(syncCommands, instruction)
}
