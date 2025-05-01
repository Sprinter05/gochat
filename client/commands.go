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

// Map that contains every shell command with its respective execution functions
var clientCmds = map[string]func(data *Data, args [][]byte) error{
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
	"REG":     reg,
	"LOGIN":   login,
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

// Sends a REQ packet to the server
// TODO
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
	return wErr
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
	shellPrint("\n", *data)

	fmt.Print("repeat password: ")
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		fmt.Print("\n")
		return pass2Err
	}
	shellPrint("\n", *data)

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
	reply, REGErr := ListenResponse(*data, 1, spec.OK, spec.ERR)
	if REGErr != nil {
		return REGErr
	}

	if reply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received (ID %d): %s", reply.HD.Info, spec.ErrorCodeToError(reply.HD.Info))
	}

	// Creates the user
	insertErr := AddLocalUser(data.DB, string(username), string(hashPass), string(prvKeyPEM), *data)
	if insertErr != nil {
		return insertErr
	}

	shellPrint(fmt.Sprintf("user %s successfully added to the database\n", username), *data)
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
	LOGINPct, LOGINPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if LOGINPctErr != nil {
		return LOGINPctErr
	}

	if data.Verbose {
		packetPrint(LOGINPct)
	}

	// Sends the packet
	_, LOGINwErr := data.ClientCon.Conn.Write(LOGINPct)
	if LOGINwErr != nil {
		return LOGINwErr
	}

	verbosePrint("[...] awaiting response...", *data)
	LOGINReply, LOGINReplyErr := ListenResponse(*data, 1, spec.ERR, spec.VERIF)
	if LOGINReplyErr != nil {
		return LOGINReplyErr
	}

	if LOGINReply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on LOGIN reply (ID %d): %s", LOGINReply.HD.Info, spec.ErrorCodeToError(LOGINReply.HD.Info))
	}

	// The reply is a VERIF
	// Decrypts the message
	pKey, PEMErr := spec.PEMToPrivkey([]byte(localUser.PrvKey))
	if PEMErr != nil {
		return PEMErr
	}

	decrypted, decryptErr := spec.DecryptText([]byte(LOGINReply.Args[0]), pKey)
	if decryptErr != nil {
		return decryptErr
	}

	// Sends a reply to the VERIF packet
	VERIFPct, VERIFPctErr := spec.NewPacket(spec.VERIF, 1, spec.EmptyInfo, []byte(username), decrypted)
	if VERIFPctErr != nil {
		return VERIFPctErr
	}

	if data.Verbose {
		packetPrint(VERIFPct)
	}

	// Sends the packet
	_, VERIFwErr := data.ClientCon.Conn.Write(VERIFPct)
	if VERIFwErr != nil {
		return VERIFwErr
	}

	// Listens for response
	verbosePrint("[...] awaiting response...", *data)
	VERIFReply, VERIFReplyErr := ListenResponse(*data, 1, spec.ERR, spec.OK)
	if VERIFReplyErr != nil {
		return VERIFReplyErr
	}

	if VERIFReply.HD.Op == spec.ERR {
		return fmt.Errorf("error packet received on RECIV reply (ID %d): %s", VERIFReply.HD.Info, spec.ErrorCodeToError(VERIFReply.HD.Info))
	}

	verbosePrint("verification successful", *data)
	shellPrint(fmt.Sprintf("login successful. Welcome, %s\n", username), *data)
	data.User = localUser
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

func packetPrint(pct []byte) {
	fmt.Println("the following packet is about to be sent:")
	VERIFcmd := spec.ParsePacket(pct)
	VERIFcmd.Print()
}
