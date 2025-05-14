package commands

import (
	"fmt"
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

// TODO: Change to return error
// Listens for an OK packet from the server when starting the connection,
// which determines that the client/server was started successfully
func ConnectionStart(data Command) error {
	cmd := new(spec.Command)

	// Header listen
	hdErr := cmd.ListenHeader(data.Data.ClientCon)
	if hdErr != nil {
		return hdErr
	}

	// Header check
	chErr := cmd.HD.ClientCheck()
	if chErr != nil {
		if data.Static.Verbose {
			data.Output(cmd.Contents(), ERROR)
		}
		return chErr
	}

	// Payload listen
	pldErr := cmd.ListenPayload(data.Data.ClientCon)
	if pldErr != nil {
		return pldErr
	}

	if cmd.HD.Op == 1 {
		data.Output("successfully connected to the server", RESULT)
	} else {
		return spec.ErrorUndefined
	}

	return nil
}

// Receives a slice of command operations to listen to, then starts
// listening until a received packet fits one of the actions provided
// and returns it
func ListenResponse(data Command, id spec.ID, ops ...spec.Action) (spec.Command, error) {
	// TODO: timeouts
	var cmd spec.Command

	for !(slices.Contains(ops, cmd.HD.Op)) {
		cmd = spec.Command{}
		// Header listen
		hdErr := cmd.ListenHeader(data.Data.ClientCon)
		if hdErr != nil {
			return cmd, hdErr
		}

		// Header check
		chErr := cmd.HD.ClientCheck()
		if chErr != nil {
			if data.Static.Verbose {
				data.Output(cmd.Contents(), ERROR)
			}
			return cmd, chErr
		}

		// Payload listen
		pldErr := cmd.ListenPayload(data.Data.ClientCon)
		if pldErr != nil {
			return cmd, pldErr
		}
	}

	if data.Static.Verbose {
		fmt.Println("Packet received from server:")
		data.Output(cmd.Contents(), ERROR)
	}

	if cmd.HD.ID != id {
		return cmd, fmt.Errorf("unexpected ID received")
	}
	return cmd, nil
}
