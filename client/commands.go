package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
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

// Struct that contains all the data required for the shell to function.
// Commands may alter the data if necessary
type Data struct {
	ClientCon spec.Connection
	Verbose   bool
	ShellMode bool // If ShellMode is true, the struct belongs to the shell and the output should be printed
	DB        *gorm.DB
	Server    db.Server
	User      db.LocalUserData
}

func (data Data) isUserLoggedIn() bool {
	return data.User.User.Username != ""
}

func (data Data) isConnected() bool {
	return data.ClientCon.Conn != nil
}

// Contains data received from the reply of a command
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

// Map that contains every shell command with its respective execution functions
var clientCmds = map[string]func(data *Data, outputFunc func(text string), args ...[]byte) ReplyData{
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

// Given a string containing a command name, returns its execution function
func FetchClientCmd(op string, outputFunc func(text string)) func(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	v, ok := clientCmds[op]
	if !ok {
		outputFunc(fmt.Sprintf("%s: command not found\n", op))
		return nil
	}
	return v
}

// CLIENT COMMANDS

// Connects a client to a gochat server
func Conn(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if data.isConnected() {
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

	data.ClientCon.Conn = con
	outputFunc("succesfully connected to the server\n")
	return ReplyData{}
}

// Disconnects a client from a gochat server
func Discn(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	err := data.ClientCon.Conn.Close()
	if err != nil {
		return ReplyData{Error: err}
	}
	data.ClientCon.Conn = nil
	// Closes the shell client session
	data.User = db.LocalUserData{}
	outputFunc("sucessfully disconnected from the server\n")
	return ReplyData{}
}

// Prints the gochat version used by the client
func Ver(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	outputFunc(fmt.Sprintf("gochat version %d\n", spec.ProtocolVersion))
	return ReplyData{}
}

// Switches on/off the verbose mode
func Verbose(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	data.Verbose = !data.Verbose
	if data.Verbose {
		outputFunc("verbose mode on\n")
	} else {
		outputFunc("verbose mode off\n")
	}
	return ReplyData{}
}

// Requests the information of an external user to add it to the client database
func Req(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.isUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, regErr := ListenResponse(*data, 1, spec.REQ, spec.ERR)
	if regErr != nil {
		return ReplyData{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	dbErr := db.AddExternalUser(data.DB, string(reply.Args[0]), string(reply.Args[1]), data.Server.ServerID)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}
	outputFunc(fmt.Sprintf("user %s successfully added to the database\n", args[0]))
	return ReplyData{Arguments: reply.Args}
}

// Registers a user to a server and also adds it to the client database
func Reg(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.isUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	rd := bufio.NewReader(os.Stdin)

	// Gets the username
	outputFunc("username: ")
	username, readErr := rd.ReadBytes('\n')
	if readErr != nil {
		return ReplyData{Error: readErr}
	}

	// Removes unecessary spaces and the line jump in the username
	username = bytes.TrimSpace(username)
	if len(username) == 0 {
		return ReplyData{Error: ErrorUsernameEmpty}
	}

	exists := db.LocalUserExists(data.DB, string(username))
	if exists {
		return ReplyData{Error: ErrorUserExists}
	}

	// Gets the password
	fmt.Print("password: ")
	pass1, pass1Err := term.ReadPassword(0)
	if pass1Err != nil {
		outputFunc("\n")
		return ReplyData{Error: pass1Err}
	}
	outputFunc("\n")

	outputFunc("repeat password: ")
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		outputFunc("\n")
		return ReplyData{Error: pass2Err}
	}
	outputFunc("\n")

	if string(pass1) != string(pass2) {
		return ReplyData{Error: ErrorPasswordsNotMatch}
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("[...] generating RSA key pair...\n", outputFunc, *data)
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
	verbosePrint("[...] hashing password...\n", outputFunc, *data)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return ReplyData{Error: hashErr}
	}

	verbosePrint("[...] sending REG packet...\n", outputFunc, *data)
	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, regErr := ListenResponse(*data, 1, spec.OK, spec.ERR)
	if regErr != nil {
		return ReplyData{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Creates the user
	insertErr := db.AddLocalUser(data.DB, string(username), string(hashPass), string(prvKeyPEM), data.Server.ServerID)
	if insertErr != nil {
		return ReplyData{Error: insertErr}
	}
	outputFunc(fmt.Sprintf("user %s successfully added to the database\n", args[0]))
	return ReplyData{Arguments: reply.Args}
}

// Logs a user to a server
func Login(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if data.isUserLoggedIn() {
		return ReplyData{Error: ErrorAlreadyLoggedIn}
	}

	username := string(args[0])
	found := db.LocalUserExists(data.DB, username)
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}

	// Asks for password
	fmt.Printf("%s's password: ", username)
	pass, passErr := term.ReadPassword(0)
	if passErr != nil {
		outputFunc("\n")
		return ReplyData{Error: passErr}
	}
	outputFunc("\n")

	// Verifies password
	localUser := db.GetLocalUser(data.DB, username)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ReplyData{Error: ErrorWrongCredentials}
	}

	verbosePrint("password correct\n[...] sending LOGIN packet...", outputFunc, *data)
	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	loginPct, loginPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if loginPctErr != nil {
		return ReplyData{Error: loginPctErr}
	}

	if data.Verbose {
		packetPrint(loginPct, outputFunc)
	}

	// Sends the packet
	_, loginWErr := data.ClientCon.Conn.Write(loginPct)
	if loginWErr != nil {
		return ReplyData{Error: loginWErr}
	}

	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
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

	if data.Verbose {
		packetPrint(verifPct, outputFunc)
	}

	// Sends the packet
	_, verifWErr := data.ClientCon.Conn.Write(verifPct)
	if verifWErr != nil {
		return ReplyData{Error: verifWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	verifReply, verifReplyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if verifReplyErr != nil {
		return ReplyData{Error: verifReplyErr}
	}

	if verifReply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(verifReply.HD.Info)}
	}
	verbosePrint("verification successful\n", outputFunc, *data)
	// Assigns the logged in user to Data
	data.User = localUser

	outputFunc(fmt.Sprintf("login successful. Welcome, %s\n", username))
	return ReplyData{Arguments: verifReply.Args}
}

// Logs out a user from a server
func Logout(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.isUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.LOGOUT, 1, spec.EmptyInfo)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, pctWErr := data.ClientCon.Conn.Write(pct)
	if pctWErr != nil {
		return ReplyData{Error: pctWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Empties the user value in Data
	data.User = db.LocalUserData{}

	outputFunc("logged out\n")
	return ReplyData{Arguments: reply.Args}
}

// Requests a list of either "online" or "all" registered users and prints it. If "local"
// is used as an argument, the local users will be printed insteads and no server requests
// will be performed
func Usrs(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.isConnected() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.isUserLoggedIn() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	var option byte
	switch string(args[0]) {
	case "online":
		option = 0x01
	case "all":
		option = 0x00
	case "local":
		outputFunc("local users:\n")
		printLocalUsers(*data, outputFunc)
		return ReplyData{}

	default:
		return ReplyData{Error: ErrorUnknownUSRSOption}
	}

	pct, pctErr := spec.NewPacket(spec.USRS, 1, option)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.USRS)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	outputFunc(fmt.Sprintf("%s users:\n", args[0]))
	outputFunc(string(reply.Args[0]))
	outputFunc("\n")
	return ReplyData{Arguments: reply.Args}
}

// Sends a message to a user with the current time stamp and stores it in the database
func Msg(data *Data, outputFunc func(text string), args ...[]byte) ReplyData {
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !data.isConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !data.isUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}
	// Stores the message before encrypting to store it in the database
	plainMessage := make([]byte, len(args[1]))
	copy(plainMessage, args[1])

	found := db.ExternalUserExists(data.DB, string(args[0]))
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}
	// Retrieves the public key in PEM format to encrypt the message
	pubKeyPEM := db.GetExternalUser(data.DB, string(args[0])).PubKey
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
	pct, pctErr := spec.NewPacket(spec.MSG, 1, spec.EmptyInfo, args[0], []byte(fmt.Sprintf("%d", stamp.Unix())), encrypted)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return ReplyData{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	outputFunc("message sent correctly\n")
	dbErr := db.StoreMessage(data.DB, string(data.User.User.Username), string(args[0]), string(plainMessage), stamp)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}
	return ReplyData{}
}

// Prints out all local users
func printLocalUsers(data Data, outputFunc func(text string)) {
	localUsers := db.GetAllLocalUsernames(data.DB)
	for i := range localUsers {
		outputFunc(fmt.Sprintf("%s\n", localUsers[i]))
	}
}

// Prints a packet
func packetPrint(pct []byte, outputFunc func(text string)) {
	fmt.Println("the following packet is about to be sent:")
	cmd := spec.ParsePacket(pct)
	cmd.Print(outputFunc)
}

// Prints text if the verbose mode is on
func verbosePrint(text string, outputFunc func(text string), data Data) {
	if data.Verbose {
		outputFunc(text)
	}
}
