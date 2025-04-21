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
var clientCmds = map[string]func(data *ShellData, args [][]byte) error{
	"VER":     ver,
	"VERBOSE": verbose,
	"REQ":     req,
	"REG":     reg,
}

// Given a string containing a command name, returns its execution function
func FetchClientCmd(op string) func(data *ShellData, args [][]byte) error {
	v, ok := clientCmds[op]
	if !ok {
		fmt.Printf("%s: command not found\n", op)
		return nil
	}
	return v
}

// CLIENT COMMANDS

// Prints the gochat version used by the client
func ver(data *ShellData, args [][]byte) error {
	fmt.Printf("gochat version %d\n", spec.ProtocolVersion)
	return nil
}

// Switches on/off the shell verbose mode
func verbose(data *ShellData, args [][]byte) error {
	data.Verbose = !data.Verbose
	if data.Verbose {
		fmt.Println("verbose mode on")
	} else {
		fmt.Println("verbose mode off")
	}
	return nil
}

// Sends a REQ packet to the server
func req(data *ShellData, args [][]byte) error {
	if len(args) < 1 {
		return fmt.Errorf("not enough arguments")
	}
	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		fmt.Println("The following packet is about to be sent:")
		cmd := spec.ParsePacket(pct)
		cmd.Print()
	}

	_, wErr := data.ClientCon.Conn.Write(pct)
	return wErr
}

func reg(data *ShellData, args [][]byte) error {
	rd := bufio.NewReader(os.Stdin)

	// Gets the username
	fmt.Print("username: ")
	username, readErr := rd.ReadBytes('\n')
	if readErr != nil {
		return readErr
	}

	// Gets the password
	fmt.Print("password: ")
	pass1, pass1Err := term.ReadPassword(0)
	if pass1Err != nil {
		fmt.Print("\n")
		return pass1Err
	}
	fmt.Print("\n")

	fmt.Print("repeat password: ")
	pass2, pass2Err := term.ReadPassword(0)
	if pass2Err != nil {
		fmt.Print("\n")
		return pass2Err
	}
	fmt.Print("\n")

	if string(pass1) != string(pass2) {
		return fmt.Errorf("passwords do not match")
	}

	// Removes unecessary spaces and the line jump in the username
	username = bytes.TrimSpace(username)
	if len(username) == 0 {
		return fmt.Errorf("username cannot be empty")
	}

	// Generates the PEM arrays of both the private and public key of the pair
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
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return hashErr
	}

	// Creates the user
	insertErr := AddLocalUser(data.DB, string(username), string(hashPass), string(prvKeyPEM), *data)
	if insertErr != nil {
		return insertErr
	}

	fmt.Printf("user %s successfully added to the database\n", username)
	fmt.Println("sending REG packet...")

	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return pctErr
	}

	if data.Verbose {
		fmt.Println("The following packet is about to be sent:")
		cmd := spec.ParsePacket(pct)
		cmd.Print()
	}

	// Sends the packet
	_, wErr := data.ClientCon.Conn.Write(pct)
	return wErr
}
