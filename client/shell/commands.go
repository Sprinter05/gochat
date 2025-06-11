package shell

// Includes auxiliary functions that sanitize the input to call the command functions
// in the commands package. It also implements aditional, shell-exclusive commands.

import (
	"fmt"
	"strconv"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

// Sets up the CONN call depending on how the user specified the server.
//
// Arguments: <server address> <server port> [-noverify] || <server name> [-noverify]
func connect(cmd commands.Command, args ...[]byte) error {
	noverify := false
	server := db.Server{}
	var dbErr error

	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	if string(args[len(args)-1]) == "-noverify" {
		noverify = true
	}

	args = args[:len(args)-1]

	// If only an argument is left, the server will be obtained by name
	if len(args) == 1 {
		server, dbErr = db.GetServerName(cmd.Static.DB, string(args[0]))
		if dbErr != nil {
			return dbErr
		}
	} else { // The server will be found by socket
		port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
		if parseErr != nil {
			return parseErr
		}

		server, dbErr = db.GetServer(cmd.Static.DB, string(args[1]), uint16(port))
		if dbErr != nil {
			return dbErr
		}
	}

	_, connErr := commands.Conn(cmd, server, noverify)
	return connErr
}

// Calls Discn, no aditional sanitization needed.
//
// Arguments: none
func disconnect(cmd commands.Command) error {
	_, discnErr := commands.Discn(cmd)
	return discnErr
}

// Prints out the gochat version used by the client.
//
// Arguments: none
func ver(cmd commands.Command, args ...[]byte) error {
	cmd.Output(
		fmt.Sprintf(
			"gochat version %d",
			spec.ProtocolVersion,
		), commands.PLAIN,
	)
	return nil
}
