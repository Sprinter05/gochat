package shell

// Includes auxiliary functions that sanitize the input to call the command functions
// in the commands package. It also implements aditional, shell-exclusive commands.

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

// Sets up the CONN call depending on how the user specified the server.
//
// Arguments: <server address> <server port> [-noverify] || <server name> [-noverify]
func connect(ctx context.Context, cmd commands.Command, args ...[]byte) error {
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
func disconnect(ctx context.Context, cmd commands.Command) error {
	_, discnErr := commands.Discn(cmd)
	return discnErr
}

// Calls REQ to request a user
// Arguments: <username to be requested> (args[0])
func requestUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}
	username := string(args[0])
	_, reqErr := commands.Req(ctx, cmd, username)
	return reqErr
}

/* SHELL-EXCLUSIVE COMMANDS */

// Prints out the gochat version used by the client.
//
// Arguments: none
func ver(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	cmd.Output(
		fmt.Sprintf(
			"gochat version %d",
			spec.ProtocolVersion,
		), commands.PLAIN,
	)
	return nil
}

// Switches on/off the verbose mode.
//
// Arguments: none
func verbose(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	cmd.Static.Verbose = !cmd.Static.Verbose

	if cmd.Static.Verbose {
		cmd.Output("verbose mode on", commands.PLAIN)
	} else {
		cmd.Output("verbose mode off", commands.PLAIN)
	}
	return nil
}
