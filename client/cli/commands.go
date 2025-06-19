package cli

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
	"time"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/term"
)

type ShellCommand struct {
	Run  func(ctx context.Context, cmd commands.Command, args ...[]byte) error
	Help string
}

var shCommands = map[string]ShellCommand{
	"CONN": {connect,
		"- CONN: Connects the client to a gochat server.\n" +
			"Usage: CONN <server address> <server port> [-noverify] || CONN <server name> [-noverify]",
	},

	"DISCN": {disconnect,
		"- DISCN: Disconnects the client to a gochat server.\n" +
			"Usage: DISCN",
	},

	"REQ": {requestUser,
		"- REQ: Requests information about a user to the gochat server.\n" +
			"Usage: REQ <username to be requested>",
	},

	"REG": {registerUser,
		"- REG: Registers a user to the gochat server the user is connected to.\n" +
			"Usage: REG",
	},

	"DEREG": {deregisterUser,
		"- DEREG: Deregisters the currently logged in user from the server.\n" +
			"Usage: DEREG",
	},

	"LOGIN": {loginUser,
		"- LOGIN: Requests information about a user to the gochat server.\n" +
			"Usage: LOGIN <username>",
	},

	"LOGOUT": {logoutUser,
		"- LOGOUT: Logs out the current user.\n" +
			"Usage: LOGOUT",
	},

	"USRS": {getUsers,
		"- USRS: Prints a list of users depending on the option provided.\n" +
			"Usage: USRS <online/all/local server/local all/requested>",
	},

	"MSG": {sendMessage,
		"- MSG: Sends a message to a user. You must REQ the user prior to sending them a message.\n" +
			"Usage: MSG <destination user> <message>",
	},

	"RECIV": {receiveMessages,
		"- RECIV: Requests a message catch-up to the gochat server.\n" +
			"Usage: RECIV",
	},

	"TLS": {changeTLS,
		"- TLS: Toggles on/off the TLS mode of the specified server.\n" +
			"Usage: TLS <on/off> <server name>",
	},

	"IMPORT": {importKey,
		". IMPORT: Imports a user to the client database provided the path of its previously-exported key.\n" +
			"Usage: IMPORT <username of the new user> <path of the key>",
	},

	"EXPORT": {exportKey,
		"- EXPORT: Exports a user.\n" +
			"Usage: EXPORT <user to be exported>",
	},

	"SUB": {subscribe,
		"- SUB: Subscribes a user to the specified hook. The user automatically unsubscribes from the hook in each disconnection.\n" +
			"Usage: SUB <all/new_login/new_logout/duplicated_session/permissions_change>",
	},

	"UNSUB": {unsubscribe,
		"-UNSUB: Unsubscribes a user from the specified hook.\n" +
			"Usage: UNSUB <all/new_login/new_logout/duplicated_session/permissions_change>",
	},

	"VER": {ver,
		"- VER: Prints the current client gochat protocol version.\n" +
			"Usage: VER",
	},

	"VERBOSE": {verbose,
		"- VERBOSE: Switches on/off the verbose mode.\n" +
			"Usage: VERBOSE",
	},

	"REGSERVER": {registerServer,
		"- REGSERVER: Registers a server to the client database.\n" +
			"Usage: REGSERVER <name> <address> <port> [-tls]",
	},

	"SERVERS": {servers,
		"- SERVERS: Prints the registered servers of the client database.\n" +
			"Usage: SERVERS"},

	"ADMIN": {sendAdminCommand,
		"- ADMIN: Sends an administrator command to the server. The user must have permissions to do so.\n" +
			"Usage: ADMIN <shtdwn/dereg/brdcast/chperms/kick> <args>"},
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

		exists, _ := db.ServerExists(cmd.Static.DB, address, uint16(port))
		if !exists {
			// If the server does not exist and you connect by socket,
			// it creates it.
			server, dbErr = db.AddServer(cmd.Static.DB, address, uint16(port), "", false)
			if dbErr != nil {
				return dbErr
			}
		} else {
			server, dbErr = db.GetServer(cmd.Static.DB, address, uint16(port))
			if dbErr != nil {
				return dbErr
			}
		}
	}

	_, connErr := commands.Conn(cmd, server, noverify)
	if connErr != nil {
		return connErr
	}
	cmd.Data.Server = &server
	go commands.ListenPackets(cmd, func() {})
	return nil
}

// Calls Discn, no aditional sanitization needed.
//
// Arguments: none
func disconnect(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	_, discnErr := commands.Discn(cmd)
	cmd.Data.Server = nil
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
	pass1, pass1Err := term.ReadPassword(int(os.Stdin.Fd()))
	if pass1Err != nil {
		cmd.Output("", commands.PROMPT)
		return pass1Err
	}
	cmd.Output("\n", commands.PROMPT)

	cmd.Output("repeat password: ", commands.PROMPT)
	pass2, pass2Err := term.ReadPassword(int(os.Stdin.Fd()))
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

// Deregisters the current user if the password verification is passed
//
// Arguments: <username to be deregistered>
func deregisterUser(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	// Asks for password
	cmd.Output(fmt.Sprintf("%s's password: ", cmd.Data.User.User.Username), commands.PROMPT)
	pass, passErr := term.ReadPassword(int(os.Stdin.Fd()))

	if passErr != nil {
		cmd.Output("\n", commands.PROMPT)
		return passErr
	}
	cmd.Output("\n", commands.PROMPT)

	_, deregErr := commands.Dereg(ctx, cmd, cmd.Data.User.User.Username, string(pass))
	if deregErr != nil {
		return deregErr
	}
	// Empties the user value in Data
	cmd.Data.User = nil
	return nil
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

	username := string(args[0])
	exists, _ := db.LocalUserExists(cmd.Static.DB, username, cmd.Data.Server.Address, cmd.Data.Server.Port)
	if !exists {
		return commands.ErrorUserNotFound
	}

	// Asks for password
	cmd.Output(fmt.Sprintf("%s's password: ", username), commands.PROMPT)
	pass, passErr := term.ReadPassword(int(os.Stdin.Fd()))

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
			if !cmd.Data.IsConnected() {
				return commands.ErrorNotConnected
			}
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
// Arguments: <username> <path>
func importKey(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 2 {
		return commands.ErrorInsuficientArgs
	}

	username := string(args[0])
	path := string(args[1])

	// Gets the password
	cmd.Output("password: ", commands.PROMPT)
	pass1, pass1Err := term.ReadPassword(int(os.Stdin.Fd()))
	if pass1Err != nil {
		cmd.Output("", commands.PROMPT)
		return pass1Err
	}
	cmd.Output("\n", commands.PROMPT)

	cmd.Output("repeat password: ", commands.PROMPT)
	pass2, pass2Err := term.ReadPassword(int(os.Stdin.Fd()))
	if pass2Err != nil {
		cmd.Output("\n", commands.PROMPT)
		return pass2Err
	}
	cmd.Output("\n", commands.PROMPT)

	if string(pass1) != string(pass2) {
		return commands.ErrorPasswordsDontMatch
	}

	_, importErr := commands.Import(cmd, username, string(pass1), path)
	return importErr
}

// Calls Export to import a key.
//
// Arguments: <username>
func exportKey(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	if !cmd.Data.IsConnected() {
		return commands.ErrorNotConnected
	}

	username := string(args[0])
	// Asks for password
	cmd.Output(fmt.Sprintf("%s's password: ", username), commands.PROMPT)
	pass, passErr := term.ReadPassword(int(os.Stdin.Fd()))

	if passErr != nil {
		cmd.Output("\n", commands.PROMPT)
		return passErr
	}
	cmd.Output("\n", commands.PROMPT)

	_, exportErr := commands.Export(cmd, username, string(pass))
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
	if tlsErr != nil {
		return tlsErr
	}

	onString := "off"
	if on {
		onString = "on"
	}

	cmd.Output(fmt.Sprintf("TLS mode turned %s for %s server", onString, server.Name), commands.RESULT)
	return nil
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

func servers(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	servers, dbErr := db.GetAllServers(cmd.Static.DB)
	if dbErr != nil {
		return dbErr
	}

	fmt.Println("all servers:")

	for _, v := range servers {
		fmt.Printf("- %s (%s:%d)\n", v.Name, v.Address, v.Port)
	}

	return nil
}

func help(cmd commands.Command, args ...[]byte) error {
	if len(args) == 0 {
		fmt.Println("To exit the shell type EXIT")
		fmt.Println()

		for _, v := range shCommands {
			fmt.Println(v.Help)
			fmt.Println()
		}

		return nil
	}

	shCmd := fetchCommand(string(args[0]), cmd)
	fmt.Println(shCmd.Help)
	return nil
}

func sendAdminCommand(ctx context.Context, cmd commands.Command, args ...[]byte) error {
	if len(args) < 1 {
		return commands.ErrorInsuficientArgs
	}

	opStr := strings.ToUpper(string(args[0]))
	adminArgs := make([][]byte, 0)
	var op spec.Admin
	switch opStr {
	case "SHTDWN":
		if len(args) < 2 {
			return commands.ErrorInsuficientArgs
		}

		op = spec.AdminShutdown
		seconds, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
		if parseErr != nil {
			return parseErr
		}
		stamp := time.Now().Add(time.Duration(seconds))
		adminArgs = append(adminArgs, spec.UnixStampToBytes(stamp))
	case "DEREG":
		if len(args) < 2 {
			return commands.ErrorInsuficientArgs
		}
		op = spec.AdminDeregister
		adminArgs = append(adminArgs, args[1])
	case "BRDCAST":
		if len(args) < 2 {
			return commands.ErrorInsuficientArgs
		}
		op = spec.AdminBroadcast
		adminArgs = append(adminArgs, args[1])
	case "CHGPERMS":
		if len(args) < 3 {
			return commands.ErrorInsuficientArgs
		}
		op = spec.AdminChangePerms
		adminArgs = append(adminArgs, args[1])
		perm, parseErr := strconv.ParseUint(string(args[2]), 10, 16)
		if parseErr != nil {
			return parseErr
		}

		permArg := make([]byte, 0)
		permArg = append(permArg, byte(perm))
		adminArgs = append(adminArgs, permArg)
	case "KICK":
		if len(args) < 2 {
			return commands.ErrorInsuficientArgs
		}
		op = spec.AdminDisconnect
		adminArgs = append(adminArgs, args[1])
	case "MOTD":
		if len(args) < 1 {
			return commands.ErrorInsuficientArgs
		}
		op = spec.AdminMotd
		adminArgs = append(adminArgs, args[1])
	default:
		return commands.ErrorInvalidAdminOperation
	}

	_, adminErr := commands.Admin(ctx, cmd, op, adminArgs...)
	return adminErr
}
