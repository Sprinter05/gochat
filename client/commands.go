package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

// TODO: PENDING and packet buffer
// TODO: CHANGESV command
// TODO: cache requested users in memory
// TODO: Print local users

// Map that contains every shell command with its respective execution functions
var clientCmds = map[string]func(data *Data, args [][]byte) error{
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
	"REG":     reg,
	"LOGIN":   login,
	"LOGOUT":  logout,
	"USRS":    usrs,
}

// Given a string containing a command name, returns its execution function
func FetchClientCmd(op string) func(data *Data, args [][]byte) error {
	v, ok := clientCmds[op]
	if !ok {
		fmt.Printf("%s: command not found\n", op)
		return nil
	}
	return v
}

// CLIENT COMMANDS

// Prints the gochat version used by the client
func ver(data *Data, args [][]byte) error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)
	return nil
}

// Switches on/off the shell verbose mode
func verbose(data *Data, args [][]byte) error {
	data.Verbose = !data.Verbose
	if data.Verbose {
		fmt.Println("verbose mode on")
	} else {
		fmt.Println("verbose mode off")
	}
	return nil
}

// Sends a REQ packet to the server and stores the received user in the database
func req(data *Data, args [][]byte) error {
	if len(args) < 1 {
		return fmt.Errorf("not enough arguments")
	}
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		packetPrint(pct)
	}

	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...", *data)
	reply, regErr := ListenResponse(*data, 1, spec.REQ, spec.ERR)
	if regErr != nil {
		return regErr
	}

	if reply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))
	}

	dbErr := AddExternalUser(data.DB, string(reply.Args[0]), string(reply.Args[1]), *data)
	if dbErr != nil {
		return dbErr
	}
	shellPrint(fmt.Sprintf("user %s successfully added to the database", args[0]), *data)
	return nil
}

func reg(data *Data, args [][]byte) error {

	rd := bufio.NewReader(os.Stdin)

	// Gets the username
	fmt.Print("username: ")
	username, readErr := rd.ReadBytes('\n')
	if readErr != nil {
		return readErr
	}

	// Removes unecessary spaces and the line jump in the username
	username = bytes.TrimSpace(username)
	if len(username) == 0 {
		return fmt.Errorf("username cannot be empty")
	}

	exists := LocalUserExists(data.DB, string(username))
	if exists {
		return fmt.Errorf("user already exists")
	}

	// Gets the password
	fmt.Print("password: ")
	pass1, pass1Err := term.ReadPassword(0)
	if pass1Err != nil {
		fmt.Print("\n")
		return pass1Err
	}
	shellPrint("", *data)

	fmt.Print("repeat password: ")
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		fmt.Print("\n")
		return pass2Err
	}
	shellPrint("", *data)

	if string(pass1) != string(pass2) {
		return fmt.Errorf("passwords do not match")
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("[...] generating RSA key pair...", *data)
	pair, rsaErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if rsaErr != nil {
		return rsaErr
	}
	prvKeyPEM := spec.PrivkeytoPEM(pair)
	pubKeyPEM, pubKeyPEMErr := spec.PubkeytoPEM(&pair.PublicKey)
	if pubKeyPEMErr != nil {
		return pubKeyPEMErr
	}

	// Hashes the provided password
	verbosePrint("[...] hashing password...", *data)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return hashErr
	}

	verbosePrint("[...] sending REG packet...", *data)
	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		packetPrint(pct)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	// Awaits a response
	verbosePrint("[...] awaiting response...", *data)
	reply, regErr := ListenResponse(*data, 1, spec.OK, spec.ERR)
	if regErr != nil {
		return regErr
	}

	if reply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))
	}

	// Creates the user
	insertErr := AddLocalUser(data.DB, string(username), string(hashPass), string(prvKeyPEM), *data)
	if insertErr != nil {
		return insertErr
	}

	shellPrint(fmt.Sprintf("user %s successfully added to the database", username), *data)
	return nil
}

func login(data *Data, args [][]byte) error {
	if len(args) < 1 {
		return fmt.Errorf("not enough arguments")
	}

	username := string(args[0])
	found := LocalUserExists(data.DB, username)
	if !found {
		return fmt.Errorf("username not found")
	}

	// Asks for password
	fmt.Printf("%s's password: ", username)
	pass, passErr := term.ReadPassword(0)
	if passErr != nil {
		fmt.Print("\n")
		return passErr
	}
	shellPrint("\n", *data)

	// Verifies password
	localUser := GetLocalUser(data.DB, username)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return fmt.Errorf("wrong credentials")
	}

	verbosePrint("password correct\n[...] sending LOGIN packet...", *data)
	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	loginPct, loginPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if loginPctErr != nil {
		return loginPctErr
	}

	if data.Verbose {
		packetPrint(loginPct)
	}

	// Sends the packet
	_, loginWErr := data.ClientCon.Conn.Write(loginPct)
	if loginWErr != nil {
		return loginWErr
	}

	verbosePrint("[...] awaiting response...", *data)
	loginReply, loginReplyErr := ListenResponse(*data, 1, spec.ERR, spec.VERIF)
	if loginReplyErr != nil {
		return loginReplyErr
	}

	if loginReply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on LOGIN reply (ID %d): %s", loginReply.HD.Info, spec.ErrorCodeToError(loginReply.HD.Info))
	}

	// The reply is a VERIF
	// Decrypts the message
	pKey, pemErr := spec.PEMToPrivkey([]byte(localUser.PrvKey))
	if pemErr != nil {
		return pemErr
	}

	decrypted, decryptErr := spec.DecryptText([]byte(loginReply.Args[0]), pKey)
	if decryptErr != nil {
		return decryptErr
	}

	// Sends a reply to the VERIF packet
	verifPct, verifPctErr := spec.NewPacket(spec.VERIF, 1, spec.EmptyInfo, []byte(username), decrypted)
	if verifPctErr != nil {
		return verifPctErr
	}

	if data.Verbose {
		packetPrint(verifPct)
	}

	// Sends the packet
	_, verifWErr := data.ClientCon.Conn.Write(verifPct)
	if verifWErr != nil {
		return verifWErr
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", *data)
	verifReply, verifReplyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if verifReplyErr != nil {
		return verifReplyErr
	}

	if verifReply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on RECIV reply (ID %d): %s", verifReply.HD.Info, spec.ErrorCodeToError(verifReply.HD.Info))
	}

	verbosePrint("verification successful", *data)
	shellPrint(fmt.Sprintf("login successful. Welcome, %s", username), *data)
	data.User = localUser
	return nil
}

func logout(data *Data, args [][]byte) error {
	if data.User.User.Username == "" {
		return fmt.Errorf("cannot log out because there is no logged in user")
	}

	pct, pctErr := spec.NewPacket(spec.LOGOUT, 1, spec.EmptyInfo)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		packetPrint(pct)
	}

	// Sends the packet
	_, pctWErr := data.ClientCon.Conn.Write(pct)
	if pctWErr != nil {
		return pctWErr
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if replyErr != nil {
		return replyErr
	}

	if reply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on LOGOUT reply (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))
	}

	// Empties the user value in Data
	data.User = LocalUserData{}
	shellPrint("logged out", *data)
	return nil
}

// Prints information in stdout if ShellMode is on
func shellPrint(info string, data Data) {
	if data.ShellMode {
		fmt.Println(info)
	}
}

func verbosePrint(info string, data Data) {
	if data.Verbose {
		shellPrint(info, data)
	}
}

func usrs(data *Data, args [][]byte) error {
	if len(args) < 1 {
		return fmt.Errorf("not enough arguments")
	}

	var option byte
	switch string(args[0]) {
	case "online":
		option = 0x01
	case "all":
		option = 0x00
	case "local":
		shellPrint("local users:", *data)
		printLocalUsers(*data)
		return nil

	default:
		return fmt.Errorf("unknown option. make sure the option is either 'online' or 'all'")
	}

	pct, pctErr := spec.NewPacket(spec.USRS, 1, option)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		packetPrint(pct)
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", *data)
	reply, replyErr := ListenResponse(*data, 1, spec.ERR, spec.USRS)
	if replyErr != nil {
		return replyErr
	}

	if reply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on USRS reply (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))
	}

	if option == 0x01 {
		shellPrint("online users:", *data)
	} else {
		shellPrint("all users:", *data)
	}
	shellPrint(string(reply.Args[0]), *data)
	return nil
}

func printLocalUsers(data Data) {
	localUsers := GetAllLocalUsernames(data.DB)
	for i := range localUsers {
		shellPrint(localUsers[i], data)
	}
}

func packetPrint(pct []byte) {
	fmt.Println("the following packet is about to be sent:")
	cmd := spec.ParsePacket(pct)
	cmd.Print()
}
