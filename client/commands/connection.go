package commands

import (
	"fmt"
	"log"
	"net"
	"slices"
	"strconv"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Connects to the gochat server given its address and port
func Connect(address string, port uint16) (net.Conn, error) {
	socket := net.JoinHostPort(address, strconv.FormatUint(uint64(port), 10))
	con, conErr := net.Dial("tcp4", socket)
	if conErr != nil {
		return nil, conErr
	}
	return con, conErr
}

/*
// Listens for incoming server packets. It also executes
// the appropiate client actions depending on the packet received
func Listen(data *Data) {
	defer data.ClientCon.Conn.Close()

	for {
		cmd := spec.Command{}

		// Header listen
		hdErr := cmd.ListenHeader(data.ClientCon)
		if hdErr != nil {
			continue
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			fmt.Print("\r\033[K")
			fmt.Println("invalid header received from server:")
			if data.Verbose {
				cmd.Print(ShellPrint)
			}
			fmt.Print("gochat() > ")
			continue
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.ClientCon)
		if pldErr != nil {
			continue
		}

		fmt.Print("\r\033[K")
		fmt.Println("Packet received from server:")
		if data.Verbose {
			cmd.Print(ShellPrint)
		}
		PrintPrompt(*data)
	}
}
*/

// Listens for an OK packet from the server when starting the connection,
// which determines that the client/server was started successfully
func ConnectionStart(data Data, outputFunc func(text string)) {

	cmd := spec.Command{}

	// Header listen
	hdErr := cmd.ListenHeader(data.ClientCon)
	if hdErr != nil {
		log.Fatal("could not connect to server: invalid header received")
	}

	// Header check
	chErr := cmd.HD.ClientCheck()
	if chErr != nil {
		if data.Verbose {
			cmd.Print(outputFunc)
		}
		log.Fatal("could not connect to server: malformed header received")
	}

	if cmd.HD.Op == 1 {
		fmt.Println("successfully connected to the server")
	} else {
		log.Fatal("could not connect to server: unexpected action code received")
	}
}

// Receives a slice of command operations to listen to, then starts
// listening until a received packet fits one of the actions provided
// and returns it
func ListenResponse(data Data, id spec.ID, ops ...spec.Action) (spec.Command, error) {
	// TODO: timeouts
	var cmd spec.Command

	for !(slices.Contains(ops, cmd.HD.Op)) {
		cmd = spec.Command{}
		// Header listen
		hdErr := cmd.ListenHeader(data.ClientCon)
		if hdErr != nil {
			return cmd, hdErr
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			if data.Verbose {
				cmd.Print(data.Output)
			}
			return cmd, chErr
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.ClientCon)
		if pldErr != nil {
			return cmd, pldErr
		}
	}

	if data.Verbose {
		fmt.Println("Packet received from server:")
		cmd.Print(data.Output)
	}

	if cmd.HD.ID != id {
		return cmd, fmt.Errorf("unexpected ID received")
	}
	return cmd, nil
}
