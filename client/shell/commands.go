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
	"strings"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/term"
)

var shCommands = map[string]func(ctx context.Context, cmd commands.Command, args ...[]byte) error{
	"CONN":      connect,
	"DISCN":     disconnect,
	"REQ":       requestUser,
	"REG":       registerUser,
	"LOGIN":     loginUser,
	"LOGOUT":    logoutUser,
	"USRS":      getUsers,
	"MSG":       sendMessage,
	"RECIV":     receiveMessages,
	"TLS":       changeTLS,
	"IMPORT":    importKey,
	"EXPORT":    exportKey,
	"SUB":       subscribe,
	"UNSUB":     unsubscribe,
	"VER":       ver,
	"VERBOSE":   verbose,
	"REGSERVER": registerServer,
}

// Sets up the CONN call depending on how the user specified the server.
//
// Arguments: <server address> <server port> [-noverify] || <server name> [-noverify]
func connect(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	noverify := false
	var server db.Server
	var dbErr error

	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	if string(args[len(args)-1]) == "-noverify" {
		noverify = true
		args = args[:len(args)-1]
	}

	// If only an argument is left, the server will be obtained by name
	if len(args) == 1 {
		name := string(args[0])
		server, dbErr = db.GetServerByName(cmd.Static.DB, name)
		if dbErr != nil {
			return dbErr
		}
	} else { // The server will be found by socket
		port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
		if parseErr != nil {
			return parseErr
		}

		address := string(args[0])
		server, dbErr = db.GetServer(cmd.Static.DB, address, uint16(port))
		if dbErr != nil {
			return dbErr
		}
	}

	_, connErr := commands.Conn(cmd, server, noverify)
	cmd.Data.Server = &server
	go commands.Listen(cmd, func() {})
	return connErr
}

// Calls Discn, no aditional sanitization needed.
//
// Arguments: none
func disconnect(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	_, discnErr := commands.Discn(cmd)
	return discnErr
}

// Calls REQ to request a user.
//
// Arguments: <username to be requested>
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

// Sanitizes the USRS option received in the argument in order to call
// the USRS command.
//
// Arguments: <online/all/local>
func getUsers(ctx context.Context, cmd commands.Command, args ...[]byte) error {

	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	var option commands.USRSType
	strOption := strings.ToUpper(string(args[0]))
	switch strOption {
	case "ONLINE":
		option = commands.ONLINE
	case "ALL":
		option = commands.ALL
	case "LOCAL":
		if len(args) < 2 {
			return commands.ErrorInsuficientArgs
		}
		localOption := strings.ToUpper(string(args[1]))

		switch localOption {
		case "SERVER":
			option = commands.LOCAL_SERVER
		case "ALL":
			option = commands.LOCAL_ALL
		default:
			return commands.ErrorUnknownUSRSOption
		}
	case "REQUESTED":
		option = commands.REQUESTED

	default:
		return commands.ErrorUnknownUSRSOption
	}

	_, usrsErr := commands.Usrs(ctx, cmd, option)
	return usrsErr
}

// Calls MSG, to send a message to a user.
// TODO: in order to send more complex messages,
// some sort of prompt should be used.
//
// Arguments: <dest. username> <unencyrpted text message>
func sendMessage(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	dstUser := string(args[0])
	plainText := string(args[1])

	_, msgErr := commands.Msg(ctx, cmd, dstUser, plainText)
	return msgErr
}

// Calls Reciv, no aditional sanitization needed.
//
// Arguments: none
func receiveMessages(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	_, recivErr := commands.Reciv(ctx, cmd)
	return recivErr
}

// Calls Sub to subscribe to a hook
//
// Arguments: <hook>
func subscribe(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	hook := string(args[0])
	_, subErr := commands.Sub(ctx, cmd, hook)
	return subErr
}

// Calls Unsub to subscribe to a hook
//
// Arguments: <hook>
func unsubscribe(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	hook := string(args[0])
	_, unsubErr := commands.Unsub(ctx, cmd, hook)
	return unsubErr
}

// Calls Import to import a key.
//
// Arguments: <username> <path> <password>
func importKey(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 3 {
		return commands.ErrorInsuficientArgs
	}

	username := string(args[0])
	pass := string(args[1])
	path := string(args[2])

	_, importErr := commands.Import(cmd, username, pass, path)
	return importErr
}

// Calls Import to import a key.
//
// Arguments: <username> <path> <password>
func exportKey(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 2 {
		return commands.ErrorInsuficientArgs
	}

	username := string(args[0])
	pass := string(args[1])

	_, exportErr := commands.Export(cmd, username, pass)
	return exportErr
}

// Calls TLS in order to switch on/off.
// Arguments after <on/off> are used to select the server to switch its mode of
//
// Arguments: <on/off> <server name> || <on/off> <server address> <port>
func changeTLS(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 2 {
		return commands.ErrorInsuficientArgs
	}

	on := false
	if strings.ToUpper(string(args[0])) == "ON" {
		on = true
	}

	var server db.Server
	var dbErr error
	if len(args) == 2 {
		name := string(args[1])
		server, dbErr = db.GetServerByName(cmd.Static.DB, name)
		if dbErr != nil {
			return dbErr
		}
	} else {
		address := string(args[1])
		port, parseErr := strconv.ParseUint(string(args[2]), 10, 16)
		if parseErr != nil {
			return parseErr
		}

		server, dbErr = db.GetServer(cmd.Static.DB, address, uint16(port))
		if dbErr != nil {
			return dbErr
		}
	}

	_, tlsErr := commands.TLS(cmd, &server, on)
	return tlsErr
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

// Adds a server to the database, with TLS off by default.
//
// Arguments: <name> <address> <port> [-tls]
func registerServer(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 3 {
		return commands.ErrorInsuficientArgs
	}

	on := false
	if len(args) == 4 && string(args[3]) == "-tls" {
		on = true
	}

	name := string(args[0])
	address := string(args[1])
	port, parseErr := strconv.ParseUint(string(args[2]), 10, 16)
	if parseErr != nil {
		return parseErr
	}

	server, dbErr := db.AddServer(cmd.Static.DB, address, uint16(port), name, on)
	if dbErr != nil {
		return dbErr
	}
	cmd.Output(fmt.Sprintf("Server %s (%s:%d) succesfully registered",
		server.Name,
		server.Address,
		server.Port,
	),
		commands.RESULT,
	)
	return nil
}
