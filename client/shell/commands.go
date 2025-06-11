package shell

// Includes auxiliary functions that sanitize the input to call the command functions
// in the commands package. It also implements aditional, shell-exclusive commands.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/term"
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

// Calls REQ to request a user.
//
// Arguments: <username to be requested> (args[0])
func requestUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}
	username := string(args[0])
	_, reqErr := commands.Req(ctx, cmd, username)
	return reqErr
}

// Opens a few prompts for the user to provide the user data and then
// registers said user with a REG call.
//
// Arguments: none
func registerUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if !cmd.Data.IsConnected() {
		return commands.ErrorNotConnected
	}

	rd := bufio.NewReader(os.Stdin)

	// Gets the username
	cmd.Output("username: ", commands.PROMPT)
	username, readErr := rd.ReadBytes('\n')
	if readErr != nil {
		return readErr
	}

	// Removes unecessary spaces and the line jump in the username
	username = bytes.TrimSpace(username)
	if len(username) == 0 {
		return commands.ErrorUsernameEmpty
	}

	exists, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		string(username),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)

	if existsErr != nil {
		return existsErr
	}
	if exists {
		return commands.ErrorUserExists
	}

	// Gets the password
	cmd.Output("password: ", commands.PROMPT)
	pass1, pass1Err := term.ReadPassword(0)
	if pass1Err != nil {
		cmd.Output("", commands.PROMPT)
		return pass1Err
	}
	cmd.Output("\n", commands.PROMPT)

	cmd.Output("repeat password: ", commands.PROMPT)
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		cmd.Output("\n", commands.PROMPT)
		return pass2Err
	}
	cmd.Output("\n", commands.PROMPT)

	if string(pass1) != string(pass2) {
		return commands.ErrorPasswordsDontMatch
	}

	_, regErr := commands.Reg(ctx, cmd, string(username), string(pass1))
	return regErr
}

// Opens a prompt to securely ask for a password in order to call the LOGIN
// command.
//
// Arguments: <username>
func loginUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if !cmd.Data.IsConnected() {
		return commands.ErrorNotConnected
	}

	if cmd.Data.IsLoggedIn() {
		return commands.ErrorAlreadyLoggedIn
	}

	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	username := args[0]
	// Asks for password
	cmd.Output(fmt.Sprintf("%s's password: ", username), commands.PROMPT)
	pass, passErr := term.ReadPassword(0)

	if passErr != nil {
		cmd.Output("\n", commands.PROMPT)
		return passErr
	}
	cmd.Output("\n", commands.PROMPT)
	_, loginErr := commands.Login(ctx, cmd, string(username), string(pass))
	return loginErr
}

// Calls Discn, no aditional sanitization needed.
//
// Arguments: none
func logoutUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	_, logoutErr := commands.Logout(ctx, cmd)
	return logoutErr
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
