package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"

	"github.com/Sprinter05/gochat/client/ui"
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
	Server    Server
	User      LocalUserData
}

// Map that contains every shell command with its respective execution functions
var clientCmds = map[string]func(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply{
	"CONN":    conn,
	"DISCN":   discn,
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
	"REG":     reg,
	"LOGIN":   login,
	"LOGOUT":  logout,
	"USRS":    usrs,
}

// Given a string containing a command name, returns its execution function
func FetchClientCmd(op string, outputFunc func(text string)) func(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	v, ok := clientCmds[op]
	if !ok {
		outputFunc(fmt.Sprintf("%s: command not found\n", op))
		return nil
	}
	return v
}

// CLIENT COMMANDS

func conn(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn != nil {
		return ui.Reply{Error: fmt.Errorf("already connected to a server")}
	}
	if len(args) < 2 {
		return ui.Reply{Error: fmt.Errorf("not enough arguments")}
	}

	port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
	if parseErr != nil {
		return ui.Reply{Error: parseErr}
	}

	con, conErr := Connect(string(args[0]), uint16(port))
	if conErr != nil {
		return ui.Reply{Error: conErr}
	}

	data.ClientCon.Conn = con
	outputFunc("succesfully connected to the server\n")
	return ui.Reply{Error: nil}
}

func discn(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn == nil {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}

	err := data.ClientCon.Conn.Close()
	if err != nil {
		return ui.Reply{Error: err}
	}
	data.ClientCon.Conn = nil
	// Closes the shell client session
	data.User = LocalUserData{}
	outputFunc("sucessfully disconnected from the server\n")
	return ui.Reply{Error: nil}
}

// Prints the gochat version used by the client
func ver(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	outputFunc(fmt.Sprintf("gochat version %d\n", spec.ProtocolVersion))
	return ui.Reply{Error: nil}
}

// Switches on/off the shell verbose mode
func verbose(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	data.Verbose = !data.Verbose
	if data.Verbose {
		outputFunc("verbose mode on\n")
	} else {
		outputFunc("verbose mode off\n")
	}
	return ui.Reply{Error: nil}
}

// Sends a REQ packet to the server and stores the received user in the database
func req(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn == nil {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}
	if len(args) < 1 {
		return ui.Reply{Error: fmt.Errorf("not enough arguments")}
	}
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return ui.Reply{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ui.Reply{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...", outputFunc, *data)
	reply, regErr := ListenResponse(*data, 1, spec.REQ, spec.ERR)
	if regErr != nil {
		return ui.Reply{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))}
	}

	dbErr := AddExternalUser(data.DB, string(reply.Args[0]), string(reply.Args[1]), *data)
	if dbErr != nil {
		return ui.Reply{Error: dbErr}
	}
	outputFunc(fmt.Sprintf("user %s successfully added to the database\n", args[0]))
	return ui.Reply{Error: nil, Arguments: reply.Args}
}

func reg(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn == nil {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}
	rd := bufio.NewReader(os.Stdin)

	// Gets the username
	outputFunc("username: \n")
	username, readErr := rd.ReadBytes('\n')
	if readErr != nil {
		return ui.Reply{Error: readErr}
	}

	// Removes unecessary spaces and the line jump in the username
	username = bytes.TrimSpace(username)
	if len(username) == 0 {
		return ui.Reply{Error: fmt.Errorf("username cannot be empty")}
	}

	exists := LocalUserExists(data.DB, string(username))
	if exists {
		return ui.Reply{Error: fmt.Errorf("user already exists")}
	}

	// Gets the password
	fmt.Print("password: ")
	pass1, pass1Err := term.ReadPassword(0)
	if pass1Err != nil {
		outputFunc("\n")
		return ui.Reply{Error: pass1Err}
	}
	outputFunc("\n")

	outputFunc("repeat password: ")
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		outputFunc("\n")
		return ui.Reply{Error: pass2Err}
	}
	outputFunc("\n")

	if string(pass1) != string(pass2) {
		return ui.Reply{Error: fmt.Errorf("passwords do not match")}
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("[...] generating RSA key pair...\n", outputFunc, *data)
	pair, rsaErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if rsaErr != nil {
		return ui.Reply{Error: rsaErr}
	}
	prvKeyPEM := spec.PrivkeytoPEM(pair)
	pubKeyPEM, pubKeyPEMErr := spec.PubkeytoPEM(&pair.PublicKey)
	if pubKeyPEMErr != nil {
		return ui.Reply{Error: pubKeyPEMErr}
	}

	// Hashes the provided password
	verbosePrint("[...] hashing password...\n", outputFunc, *data)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return ui.Reply{Error: hashErr}
	}

	verbosePrint("[...] sending REG packet...", outputFunc, *data)
	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return ui.Reply{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ui.Reply{Error: wErr}
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...", outputFunc, *data)
	reply, regErr := ListenResponse(*data, 1, spec.OK, spec.ERR)
	if regErr != nil {
		return ui.Reply{Error: regErr}
	}

	if reply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))}
	}

	// Creates the user
	insertErr := AddLocalUser(data.DB, string(username), string(hashPass), string(prvKeyPEM), *data)
	if insertErr != nil {
		return ui.Reply{Error: insertErr}
	}
	outputFunc(fmt.Sprintf("user %s successfully added to the database\n", args[0]))
	return ui.Reply{Error: nil, Arguments: reply.Args}
}

func login(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn == nil {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}
	if len(args) < 1 {
		return ui.Reply{Error: fmt.Errorf("not enough arguments")}
	}

	username := string(args[0])
	found := LocalUserExists(data.DB, username)
	if !found {
		return ui.Reply{Error: fmt.Errorf("username not found")}
	}

	// Asks for password
	fmt.Printf("%s's password: ", username)
	pass, passErr := term.ReadPassword(0)
	if passErr != nil {
		outputFunc("\n")
		return ui.Reply{Error: passErr}
	}
	outputFunc("\n")

	// Verifies password
	localUser := GetLocalUser(data.DB, username)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ui.Reply{Error: fmt.Errorf("wrong credentials")}
	}

	verbosePrint("password correct\n[...] sending LOGIN packet...", outputFunc, *data)
	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	loginPct, loginPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if loginPctErr != nil {
		return ui.Reply{Error: loginPctErr}
	}

	if data.Verbose {
		packetPrint(loginPct, outputFunc)
	}

	// Sends the packet
	_, loginWErr := data.ClientCon.Conn.Write(loginPct)
	if loginWErr != nil {
		return ui.Reply{Error: loginWErr}
	}

	verbosePrint("[...] awaiting response...", outputFunc, *data)
	loginReply, loginReplyErr := ListenResponse(*data, 1, spec.ERR, spec.VERIF)
	if loginReplyErr != nil {
		return ui.Reply{Error: loginReplyErr}
	}

	if loginReply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received on LOGIN reply (ID %d): %s", loginReply.HD.Info, spec.ErrorCodeToError(loginReply.HD.Info))}
	}

	// The reply is a VERIF
	// Decrypts the message
	pKey, pemErr := spec.PEMToPrivkey([]byte(localUser.PrvKey))
	if pemErr != nil {
		return ui.Reply{Error: pemErr}
	}

	decrypted, decryptErr := spec.DecryptText([]byte(loginReply.Args[0]), pKey)
	if decryptErr != nil {
		return ui.Reply{Error: decryptErr}
	}

	// Sends a reply to the VERIF packet
	verifPct, verifPctErr := spec.NewPacket(spec.VERIF, 1, spec.EmptyInfo, []byte(username), decrypted)
	if verifPctErr != nil {
		return ui.Reply{Error: verifPctErr}
	}

	if data.Verbose {
		packetPrint(verifPct, outputFunc)
	}

	// Sends the packet
	_, verifWErr := data.ClientCon.Conn.Write(verifPct)
	if verifWErr != nil {
		return ui.Reply{Error: verifWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", outputFunc, *data)
	verifReply, verifReplyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if verifReplyErr != nil {
		return ui.Reply{Error: verifReplyErr}
	}

	if verifReply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received on RECIV reply (ID %d): %s", verifReply.HD.Info, spec.ErrorCodeToError(verifReply.HD.Info))}
	}
	verbosePrint("verification successful\n", outputFunc, *data)
	// Assigns the logged in user to Data
	data.User = localUser

	outputFunc(fmt.Sprintf("login successful. Welcome, %s\n", username))
	return ui.Reply{Error: nil, Arguments: verifReply.Args}
}

func logout(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if data.ClientCon.Conn == nil {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}
	if data.User.User.Username == "" {
		return ui.Reply{Error: fmt.Errorf("cannot log out because there is no logged in user")}
	}

	pct, pctErr := spec.NewPacket(spec.LOGOUT, 1, spec.EmptyInfo)
	if pctErr != nil {
		return ui.Reply{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, pctWErr := data.ClientCon.Conn.Write(pct)
	if pctWErr != nil {
		return ui.Reply{Error: pctWErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...\n", outputFunc, *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return ui.Reply{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received on LOGOUT reply (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))}
	}

	// Empties the user value in Data
	data.User = LocalUserData{}

	outputFunc("logged out")
	return ui.Reply{Error: nil, Arguments: reply.Args}
}

func usrs(data *Data, outputFunc func(text string), args ...[]byte) ui.Reply {
	if len(args) < 1 {
		return ui.Reply{Error: fmt.Errorf("not enough arguments")}
	}
	if data.ClientCon.Conn == nil && !(string(args[0]) == "local") {
		return ui.Reply{Error: fmt.Errorf("not connected to a server")}
	}

	var option byte
	switch string(args[0]) {
	case "online":
		option = 0x01
	case "all":
		option = 0x00
	case "local":
		outputFunc("local users:")
		printLocalUsers(*data, outputFunc)
		return ui.Reply{Error: nil}

	default:
		return ui.Reply{Error: fmt.Errorf("unknown option. make sure the option is either 'online' or 'all'")}
	}

	pct, pctErr := spec.NewPacket(spec.USRS, 1, option)
	if pctErr != nil {
		return ui.Reply{Error: pctErr}
	}

	if data.Verbose {
		packetPrint(pct, outputFunc)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ui.Reply{Error: wErr}
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", outputFunc, *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.USRS)
	if replyErr != nil {
		return ui.Reply{Error: replyErr}
	}

	if reply.HD.Op == spec.ERR {
		return ui.Reply{Error: fmt.Errorf("error packet received on USRS reply (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))}
	}

	if option == 0x01 {
		outputFunc("online users:\n")
	} else {
		outputFunc("all users:\n")
	}
	outputFunc(string(reply.Args[0]))
	return ui.Reply{Error: nil, Arguments: reply.Args}
}

func printLocalUsers(data Data, outputFunc func(text string)) {
	localUsers := GetAllLocalUsernames(data.DB)
	for i := range localUsers {
		outputFunc(fmt.Sprintf("%s\n", localUsers[i]))
	}
}

func packetPrint(pct []byte, outputFunc func(text string)) {
	fmt.Println("the following packet is about to be sent:")
	cmd := spec.ParsePacket(pct)
	cmd.Print(outputFunc)
}

func verbosePrint(text string, outputFunc func(text string), data Data) {
	if data.Verbose {
		outputFunc(text)
	}
}
