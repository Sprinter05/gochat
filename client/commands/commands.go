package commands

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// TODO: PENDING and packet buffer
// TODO: cache requested users in memory
// TODO: USERINFO command
// TODO: HELP

// Struct that contains all the data required for the shell to function.
// Commands may alter the data if necessary.
type Data struct {
	// TODO: Thread safe??
	ClientCon spec.Connection
	Server    db.Server
	User      db.LocalUserData
}

// Separated struct that eases interaction with the terminal UI
type StaticData struct {
	Verbose bool
	DB      *gorm.DB
}

type CmdArgs struct {
	Output func(text string)
	Static StaticData
	Data   *Data
}

func (data Data) IsUserLoggedIn() bool {
	return data.User.User.Username != ""
}

func (data Data) IsConnected() bool {
	return data.ClientCon.Conn != nil
}

// Contains data received from the reply of a command.
type ReplyData struct {
	Arguments [][]byte
	Error     error
}

var (
	ErrorInsuficientArgs   error = fmt.Errorf("not enough arguments")
	ErrorNotConnected      error = fmt.Errorf("not connected to a server")
	ErrorAlreadyConnected  error = fmt.Errorf("already connected to a server")
	ErrorNotLoggedIn       error = fmt.Errorf("you are not logged in")
	ErrorAlreadyLoggedIn   error = fmt.Errorf("you are already logged in")
	ErrorWrongCredentials  error = fmt.Errorf("wrong credentials")
	ErrorUnknownUSRSOption error = fmt.Errorf("unknown option. make sure the option is either 'online' or 'all'")
	ErrorUsernameEmpty     error = fmt.Errorf("username cannot be empty")
	ErrorUserExists        error = fmt.Errorf("user exists")
	ErrorPasswordsNotMatch error = fmt.Errorf("passwords do not match")
	ErrorUserNotFound      error = fmt.Errorf("user not found")
)

// Map that contains every shell command with its respective execution functions.
var clientCmds = map[string]func(data *CmdArgs, args ...[]byte) ReplyData{
	"CONN":    Conn,
	"DISCN":   Discn,
	"VER":     Ver,
	"VERBOSE": Verbose,
	"REQ":     Req,
	"REG":     Reg,
	"LOGIN":   Login,
	"LOGOUT":  Logout,
	"USRS":    Usrs,
	"MSG":     Msg,
}

// Given a string containing a command name, returns its execution function.
func FetchClientCmd(op string, data CmdArgs) func(data *CmdArgs, args ...[]byte) ReplyData {
	v, ok := clientCmds[strings.ToUpper(op)]
	if !ok {
		data.Output(fmt.Sprintf("%s: command not found\n", op))
		return nil
	}
	return v
}

// CLIENT COMMANDS

// Starts a connection with a server.
//
// Arguments: <server address> <server port>
//
// Returns a zero value ReplyData if the connection was successful.
func Conn(data *CmdArgs, args ...[]byte) ReplyData {
	if data.Data.IsConnected() {
		return ReplyData{Error: ErrorAlreadyConnected}
	}
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
	if parseErr != nil {
		return ReplyData{Error: parseErr}
	}

	con, conErr := Connect(string(args[0]), uint16(port))
	if conErr != nil {
		return ReplyData{Error: conErr}
	}

	data.Data.ClientCon.Conn = con
	data.Data.Server.Address = string(args[0])
	data.Data.Server.Port = uint16(port)
	ConnectionStart(data)
	return ReplyData{}
}

// Disconnects a client from a gochat server.
//
// Arguments: none
//
// Returns a zero value ReplyData if the disconnection was successful.
func Discn(data *CmdArgs, args ...[]byte) ReplyData {
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	err := data.Data.ClientCon.Conn.Close()
	if err != nil {
		return ReplyData{Error: err}
	}
	data.Data.ClientCon.Conn = nil
	// Closes the shell client session
	data.Data.User = db.LocalUserData{}
	data.Output("sucessfully disconnected from the server\n")
	return ReplyData{}
}

// Prints the gochat version used by the client
func Ver(data *CmdArgs, args ...[]byte) ReplyData {
	data.Output(fmt.Sprintf("gochat version %d\n", spec.ProtocolVersion))
	return ReplyData{}
}

// Switches on/off the verbose mode.
//
// Arguments: none
//
// Returns a zero value ReplyData.
func Verbose(data *CmdArgs, args ...[]byte) ReplyData {
	data.Static.Verbose = !data.Static.Verbose
	if data.Static.Verbose {
		data.Output("verbose mode on\n")
	} else {
		data.Output("verbose mode off\n")
	}
	return ReplyData{}
}

// Requests the information of an external user to add it to the client database.
//
// Arguments: <username to be requested>
//
// Returns a ReplyData containing the reply REQ arguments.
func Req(data *CmdArgs, args ...[]byte) ReplyData {
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Static.Verbose {
		packetPrint(pct, *data)
	}

	_, wErr := data.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...\n", *data)
	reply, regErr := ListenResponse(*data, 1, spec.REQ, spec.ERR)
	if regErr != nil {
		return ReplyData{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	dbErr := db.AddExternalUser(data.Static.DB, string(reply.Args[0]), string(reply.Args[1]), data.Data.Server.ServerID)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}
	data.Output(fmt.Sprintf("user %s successfully added to the database\n", args[0]))
	return ReplyData{Arguments: reply.Args}
}

// Registers a user to a server and also adds it to the client database.
// A prompt will get the user input if the user and password is not specified.
//
// Arguments: [user] [password]
//
// Returns a zero value ReplyData if an OK packet is received after the sent REG packet.
func Reg(data *CmdArgs, args ...[]byte) ReplyData {
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) == 1 {
		return ReplyData{Error: spec.ErrorArguments}
	}
	var username []byte
	var pass1 []byte

	if len(args) == 0 {
		rd := bufio.NewReader(os.Stdin)

		// Gets the username
		data.Output("username: ")
		var readErr error
		username, readErr = rd.ReadBytes('\n')
		if readErr != nil {
			return ReplyData{Error: readErr}
		}

		// Removes unecessary spaces and the line jump in the username
		username = bytes.TrimSpace(username)
		if len(username) == 0 {
			return ReplyData{Error: ErrorUsernameEmpty}
		}

		exists := db.LocalUserExists(data.Static.DB, string(username))
		if exists {
			return ReplyData{Error: ErrorUserExists}
		}

		// Gets the password
		data.Output("password: ")
		var pass1Err error
		pass1, pass1Err = term.ReadPassword(0)
		if pass1Err != nil {
			data.Output("\n")
			return ReplyData{Error: pass1Err}
		}
		data.Output("\n")

		data.Output("repeat password: ")
		pass2, pass2Err := term.ReadPassword(0)
		if pass2Err != nil {
			data.Output("\n")
			return ReplyData{Error: pass2Err}
		}
		data.Output("\n")

		if string(pass1) != string(pass2) {
			return ReplyData{Error: ErrorPasswordsNotMatch}
		}
	} else {
		username = args[0]
		pass1 = args[1]
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("[...] generating RSA key pair...\n", *data)
	pair, rsaErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if rsaErr != nil {
		return ReplyData{Error: rsaErr}
	}
	prvKeyPEM := spec.PrivkeytoPEM(pair)
	pubKeyPEM, pubKeyPEMErr := spec.PubkeytoPEM(&pair.PublicKey)
	if pubKeyPEMErr != nil {
		return ReplyData{Error: pubKeyPEMErr}
	}

	// Hashes the provided password
	verbosePrint("[...] hashing password...\n", *data)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return ReplyData{Error: hashErr}
	}

	verbosePrint("[...] sending REG packet...\n", *data)
	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Static.Verbose {
		packetPrint(pct, *data)
	}

	// Sends the packet
	_, wErr := data.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...\n", *data)
	reply, regErr := ListenResponse(*data, 1, spec.OK, spec.ERR)
	if regErr != nil {
		return ReplyData{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Creates the user
	insertErr := db.AddLocalUser(data.Static.DB, string(username), string(hashPass), string(prvKeyPEM), data.Data.Server.ServerID)
	if insertErr != nil {
		return ReplyData{Error: insertErr}
	}
	data.Output(fmt.Sprintf("user %s successfully added to the database\n", username))
	return ReplyData{}
}

// Logs a user to a server. If only the username
// is given, the command will ask for the password.
//
// Arguments: <username> [password]
//
// Returns a zero value ReplyData if an OK packet
// is received after the sent VERIF packet.
func Login(data *CmdArgs, args ...[]byte) ReplyData {
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if data.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorAlreadyLoggedIn}
	}
	username := string(args[0])
	found := db.LocalUserExists(data.Static.DB, username)
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}

	var pass []byte
	var passErr error

	if len(args) == 1 {
		// Asks for password
		data.Output(fmt.Sprintf("%s's password: ", username))
		pass, passErr = term.ReadPassword(0)
		if passErr != nil {
			data.Output("\n")
			return ReplyData{Error: passErr}
		}
		data.Output("\n")
	} else {
		pass = args[1]
	}

	// Verifies password
	localUser := db.GetLocalUser(data.Static.DB, username, data.Data.Server.ServerID)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ReplyData{Error: ErrorWrongCredentials}
	}

	verbosePrint("password correct\n[...] sending LOGIN packet...\n", *data)
	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	loginPct, loginPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if loginPctErr != nil {
		return ReplyData{Error: loginPctErr}
	}

	if data.Static.Verbose {
		packetPrint(loginPct, *data)
	}

	// Sends the packet
	_, loginWErr := data.Data.ClientCon.Conn.Write(loginPct)
	if loginWErr != nil {
		return ReplyData{Error: loginWErr}
	}

	verbosePrint("[...] awaiting response...\n", *data)
	loginReply, loginReplyErr := ListenResponse(*data, 1, spec.ERR, spec.VERIF)
	if loginReplyErr != nil {
		return ReplyData{Error: loginReplyErr}
	}

	if loginReply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(loginReply.HD.Info)}
	}

	// The reply is a VERIF
	// Decrypts the message
	pKey, pemErr := spec.PEMToPrivkey([]byte(localUser.PrvKey))
	if pemErr != nil {
		return ReplyData{Error: pemErr}
	}

	decrypted, decryptErr := spec.DecryptText([]byte(loginReply.Args[0]), pKey)
	if decryptErr != nil {
		return ReplyData{Error: decryptErr}
	}

	// Sends a reply to the VERIF packet
	verifPct, verifPctErr := spec.NewPacket(spec.VERIF, 1, spec.EmptyInfo, []byte(username), decrypted)
	if verifPctErr != nil {
		return ReplyData{Error: verifPctErr}
	}

	if data.Static.Verbose {
		packetPrint(verifPct, *data)
	}

	// Sends the packet
	_, verifWErr := data.Data.ClientCon.Conn.Write(verifPct)
	if verifWErr != nil {
		return ReplyData{Error: verifWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", *data)
	verifReply, verifReplyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if verifReplyErr != nil {
		return ReplyData{Error: verifReplyErr}
	}

	if verifReply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(verifReply.HD.Info)}
	}
	verbosePrint("verification successful\n", *data)
	// Assigns the logged in user to Data
	data.Data.User = localUser

	data.Output(fmt.Sprintf("login successful. Welcome, %s\n", username))
	return ReplyData{Arguments: verifReply.Args}
}

// Logs out a user from a server.
//
// Arguments: none
//
// Returns a zero value ReplyData if an OK packet is received after the sent LOGOUT packet.
func Logout(data *CmdArgs, args ...[]byte) ReplyData {
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.LOGOUT, 1, spec.EmptyInfo)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Static.Verbose {
		packetPrint(pct, *data)
	}

	// Sends the packet
	_, pctWErr := data.Data.ClientCon.Conn.Write(pct)
	if pctWErr != nil {
		return ReplyData{Error: pctWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Empties the user value in Data
	data.Data.User = db.LocalUserData{}

	data.Output("logged out\n")
	return ReplyData{}
}

// Requests a list of either "online" or "all" registered users and prints it. If "local"
// is used as an argument, the local users will be printed insteads and no server requests
// will be performed.
//
// Arguments: <online/all/local>
//
// Returns a zero value ReplyData if an OK packet is received after the sent VERIF packet.
func Usrs(data *CmdArgs, args ...[]byte) ReplyData {
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.Data.IsConnected() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.Data.IsUserLoggedIn() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	var option byte
	switch string(args[0]) {
	case "online":
		option = 0x01
	case "all":
		option = 0x00
	case "local":
		data.Output("local users:\n")
		printLocalUsers(*data)
		return ReplyData{}

	default:
		return ReplyData{Error: ErrorUnknownUSRSOption}
	}

	pct, pctErr := spec.NewPacket(spec.USRS, 1, option)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Static.Verbose {
		packetPrint(pct, *data)
	}

	// Sends the packet
	_, wErr := data.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.USRS)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	data.Output(fmt.Sprintf("%s users:\n", args[0]))
	data.Output(string(reply.Args[0]))
	data.Output("\n")
	return ReplyData{Arguments: reply.Args}
}

// Sends a message to a user with the current time stamp and stores it in the database.
//
// Arguments: <dest. username> <unencyrpted text message>
//
// Returns a zero value ReplyData if an OK packet is received after the sent MSG packet
func Msg(data *CmdArgs, args ...[]byte) ReplyData {
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}
	// Stores the message before encrypting to store it in the database
	plainMessage := make([]byte, len(args[1]))
	copy(plainMessage, args[1])

	found := db.ExternalUserExists(data.Static.DB, string(args[0]))
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}
	// Retrieves the public key in PEM format to encrypt the message
	pubKeyPEM := db.GetExternalUser(data.Static.DB, string(args[0]), data.Data.Server.ServerID).PubKey
	pubKey, pemErr := spec.PEMToPubkey([]byte(pubKeyPEM))
	if pemErr != nil {
		return ReplyData{Error: pemErr}
	}
	// Encrypts the text
	encrypted, encryptErr := spec.EncryptText(args[1], pubKey)
	if encryptErr != nil {
		return ReplyData{Error: encryptErr}
	}

	stamp := time.Now()
	// Generates the packet, using the current UNIX timestamp
	pct, pctErr := spec.NewPacket(spec.MSG, 1, spec.EmptyInfo, args[0], spec.UnixStampToBytes(stamp), encrypted)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Static.Verbose {
		packetPrint(pct, *data)
	}

	// Sends the packet
	_, wErr := data.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	data.Output("message sent correctly\n")
	src := db.GetUser(data.Static.DB, data.Data.User.User.Username, data.Data.Server.ServerID)
	dst := db.GetUser(data.Static.DB, string(args[0]), data.Data.Server.ServerID)
	dbErr := db.StoreMessage(data.Static.DB, src, dst, string(plainMessage), stamp)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}
	return ReplyData{}
}

// Prints out all local users.
func printLocalUsers(data CmdArgs) {
	localUsers := db.GetAllLocalUsernames(data.Static.DB)
	for i := range localUsers {
		data.Output(fmt.Sprintf("%s\n", localUsers[i]))
	}
}

// Prints a packet.
func packetPrint(pct []byte, data CmdArgs) {
	data.Output("the following packet is about to be sent:\n")
	cmd := spec.ParsePacket(pct)
	cmd.Print(data.Output)
}

// Prints text if the verbose mode is on.
func verbosePrint(text string, data CmdArgs) {
	if data.Static.Verbose {
		data.Output(text)
	}
}
