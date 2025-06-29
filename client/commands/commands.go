package commands

// Implements the client-side functionality needed to execute requests to a gochat
// server and reacts accordinly to every server response

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/crypto/bcrypt"
)

/* CUSTOM TYPES */

// Represents the type of a command output
// This eases specific output printing actions
type OutputType uint

const (
	INTERMEDIATE OutputType = iota // Intermediate status messages
	PACKET                         // Packet information messages
	PROMPT                         // Prompt message when input is asked
	RESULT                         // Messages that show the result of a command
	ERROR                          // Error messages that may be printed additionaly in error cases
	INFO                           // Message that representes generic info not asocciated to a command
	USRSRESPONSE                   // Specific for user printing
	COLOR                          // Special output for shell colors
	PLAIN                          // Output type that should be printed as-is, with no prefix
	SECONDARY                      // Optional text to show after the result
)

// Represents the function that will be called
// when outputting information
type OutputFunc func(text string, outputType OutputType)

// Represents the different USRS command types
type USRSType uint

const (
	ALL          USRSType = 0 // Users in the server (as spec)
	ONLINE       USRSType = 1 // Online users in the server (as spec)
	ALLPERMS     USRSType = 2 // All users with perms (as spec)
	ONLINEPERMS  USRSType = 3 // Online users with perms (as spec)
	LOCAL_SERVER USRSType = 4 // Registered local users for a server
	LOCAL_ALL    USRSType = 5 // All local users
	REQUESTED    USRSType = 6 // All external users whose public key has been saved
)

/* ERRORS AND CONSTANTS */

var (
	ErrorInsuficientArgs       error = fmt.Errorf("not enough arguments")                           // not enough arguments
	ErrorNotConnected          error = fmt.Errorf("not connected to a server")                      // not connected to a server
	ErrorAlreadyConnected      error = fmt.Errorf("already connected to a server")                  // already connected to a server
	ErrorNotLoggedIn           error = fmt.Errorf("you are not logged in")                          // you are not logged in
	ErrorAlreadyLoggedIn       error = fmt.Errorf("you are already logged in")                      // you are already logged in
	ErrorWrongCredentials      error = fmt.Errorf("wrong credentials")                              // wrong credentials
	ErrorUnknownUSRSOption     error = fmt.Errorf("unknown usrs option provided")                   // unknown usrs option provided
	ErrorUsernameEmpty         error = fmt.Errorf("username cannot be empty")                       // username cannot be empty
	ErrorUserExists            error = fmt.Errorf("local user exists")                              // local user exists
	ErrorPasswordsDontMatch    error = fmt.Errorf("passwords do not match")                         // passwords do not match
	ErrorUserNotFound          error = fmt.Errorf("user not found")                                 // user not found
	ErrorUnknownTLSOption      error = fmt.Errorf("unknown tls option provided")                    // unknown tls option provided
	ErrorOfflineRequired       error = fmt.Errorf("you must be offline")                            // you must be offline
	ErrorInvalidSkipVerify     error = fmt.Errorf("cannot skip verification on a non-TLS endpoint") // cannot skip verification on a non-TLS endpoint
	ErrorRequestToSelf         error = fmt.Errorf("cannot request yourself")                        // cannot request yourself
	ErrorUnknownHookOption     error = fmt.Errorf("invalid hook provided")                          // invalid hook provided
	ErrorInvalidAdminOperation error = fmt.Errorf("invalid admin operation")                        // invalid admin operation
	ErrorRecoveryPassword      error = fmt.Errorf("could not recover during password checking")     // could not recover during password checking
	ErrorInvalidTarget         error = fmt.Errorf("provided object is not an appropiate type")      // provided object is not an appropiate type
	ErrorInvalidField          error = fmt.Errorf("provided field is non-existant")                 // provided field is non-existant
	ErrorCannotSet             error = fmt.Errorf("failed to set a value on the given field")       // failed to set a value on the given field
	ErrorNoReusableToken       error = fmt.Errorf("reusable token is empty")                        // reusable token is empty
)

// Default level of permissions that should be used
const DefaultPerms = 0755

/* LOOKUP TABLES */

// List of hooks and their names.
var hooksList = map[string]spec.Hook{
	"all":                spec.HookAllHooks,
	"new_login":          spec.HookNewLogin,
	"new_logout":         spec.HookNewLogout,
	"duplicated_session": spec.HookDuplicateSession,
	"permissions_change": spec.HookPermsChange,
}

// List of admin operations and their
// names.
var adminList = map[string]spec.Admin{
	"shutdown":  spec.AdminShutdown,
	"broadcast": spec.AdminBroadcast,
	"ban":       spec.AdminDeregister,
	"kick":      spec.AdminDisconnect,
	"setperms":  spec.AdminChangePerms,
	"motd":      spec.AdminMotd,
}

/* CLIENT COMMANDS */

// Sets a variable on an object as configuration.
// Passed objects must be pointers. Does not require
// a Data struct in "Command"
func SET(cmd Command, target, value string, objs ...ConfigObj) error {
	// We get the initial prefix
	prefix, actual, ok := strings.Cut(target, ".")
	if !ok {
		return ErrorInvalidField
	}

	found := false
	for _, v := range objs {
		// Not the object we are looking for
		if prefix != v.Prefix {
			continue
		}

		// Cannot be empty
		if prefix == "" {
			continue
		}

		// Check that the function can run
		if v.Precondition != nil {
			err := v.Precondition()
			if err != nil {
				return err
			}
		}

		// Set the value in the struct
		val, rollback, err := setConfig(v.Object, actual, value)
		if err != nil {
			return err
		}

		// Modify the database if applicable
		if v.Update != nil {
			// Used to modify the database
			column := strings.ToLower(actual)

			err := v.Update(cmd.Static.DB, v.Object, column, val)
			if err != nil {
				rollback()
				return err
			}
		}

		// Run any post hooks
		if v.Finish != nil {
			go v.Finish()
		}

		// Completed
		found = true
		break
	}

	if !found {
		return ErrorInvalidField
	}

	str := fmt.Sprintf(
		"succesfully changed %s to %s",
		target, value,
	)
	cmd.Output(str, RESULT)

	return nil
}

// Returns the current configuration values for the
// given objects. Only needs the object and prefix.
// Passed objects as "any" can or not be pointers
func CONFIG(objs ...ConfigObj) [][]byte {
	buf := make([][]byte, 0)

	for _, v := range objs {
		config, err := getConfig(v.Object, v.Prefix)
		if err == nil {
			buf = append(buf, config...)
		}
	}

	return buf
}

// Recovers the private key and messages for a specified user
// Does not require a Data struct in Command
func RECOVER(cmd Command, username, pass string, cleanup bool) error {
	verbosePrint("recovering data...", cmd)
	users, err := db.RecoverUsers(cmd.Static.DB, username)
	if err != nil {
		return err
	}

	var target db.LocalUser
	attempts := 0

	clean := func() {
		if cleanup {
			db.CleanupUser(cmd.Static.DB, target)
			cmd.Output("deleted user from database", RESULT)
		}
	}

	// We try the password on every user
	verbosePrint("trying password...", cmd)
	for _, v := range users {
		hash := []byte(v.Password)
		err := bcrypt.CompareHashAndPassword(hash, []byte(pass))
		if err != nil {
			attempts += 1
			continue
		}

		// User found
		target = v
		break
	}

	// No matching password found
	if attempts == len(users) {
		return ErrorRecoveryPassword
	}

	verbosePrint("exporting private key...", cmd)
	unamedir := path.Join("export", username+".priv")
	dec, err := db.DecryptData([]byte(pass), []byte(target.PrvKey))
	if err != nil {
		return err
	}

	err = os.WriteFile(unamedir, []byte(dec), DefaultPerms)
	if err != nil {
		return err
	}

	str1 := fmt.Sprintf(
		"file succesfully written to %s", unamedir,
	)
	cmd.Output(str1, RESULT)

	verbosePrint("exporting messages...", cmd)
	msgs, err := db.RecoverMessages(cmd.Static.DB, target)
	if err != nil {
		return err
	}

	if len(msgs) == 0 {
		cmd.Output("no messages to export", RESULT)
		clean()
		return nil
	}

	var messages strings.Builder
	for _, v := range msgs {
		messages.WriteString("--- CONVERSATION BEGINS ---\n")
		for _, m := range v {
			str := fmt.Sprintf(
				"%s | [%s] -> [%s]: %s\n",
				m.Stamp.Format(time.RFC850),
				m.SourceUser.Username,
				m.DestinationUser.Username,
				m.Text,
			)
			messages.WriteString(str)
		}
		messages.WriteString("--- CONVERSATION FINISH ---\n")
	}

	msgsdir := path.Join("export", username+".msgs")
	err = os.WriteFile(msgsdir, []byte(messages.String()), DefaultPerms)
	if err != nil {
		return err
	}

	str2 := fmt.Sprintf(
		"file succesfully written to %s", msgsdir,
	)
	cmd.Output(str2, RESULT)

	clean()
	return nil
}

// Imports a private RSA key for a new local user
// from the "import" directory using the specification PEM format.
func IMPORT(cmd Command, username, pass, dir string) error {
	// Creates import/ directory if it does not exist
	if _, err := os.Stat("import"); errors.Is(err, fs.ErrNotExist) {
		cmd.Output("missing 'import' folder", ERROR)
		return err
	}

	verbosePrint("reading private key...", cmd)
	fulldir := path.Join("import", dir)
	buf, readErr := os.ReadFile(fulldir)
	if readErr != nil {
		return readErr
	}

	_, chkErr := spec.PEMToPrivkey(buf)
	if chkErr != nil {
		return chkErr
	}

	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr != nil {
		return hashErr
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, encryptErr := db.EncryptData([]byte(pass), buf)
	if encryptErr != nil {
		return encryptErr
	}

	_, insertErr := db.AddLocalUser(
		cmd.Static.DB,
		string(username),
		string(hashPass),
		string(enc),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if insertErr != nil {
		return insertErr
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return nil
}

// Exports a local user as a private RSA key
// in the "export" folder using the spec PEM format.
func EXPORT(cmd Command, username, pass string) error {
	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return existsErr
	}
	if !found {
		return ErrorUserNotFound
	}

	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return localUserErr
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return ErrorWrongCredentials
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, decryptErr := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if decryptErr != nil {
		return decryptErr
	}

	if _, err := os.Stat("export"); errors.Is(err, fs.ErrNotExist) {
		cmd.Output("missing 'export' directory", ERROR)
		return err
	}

	fulldir := path.Join("export", username+".priv")
	writeErr := os.WriteFile(fulldir, []byte(dec), DefaultPerms)
	if writeErr != nil {
		return writeErr
	}

	str := fmt.Sprintf(
		"file succesfully written to %s", fulldir,
	)
	cmd.Output(str, RESULT)
	return nil
}

// Starts a connection with a server. If noverify is set,
// in case of TLS connections, certificate origins wont be checked.
// This command does not spawn a listening thread.
func CONN(cmd Command, server db.Server, noverify bool) error {
	if cmd.Data.IsConnected() {
		return ErrorAlreadyConnected
	}

	useTLS := server.TLS
	skipVerify := false

	if noverify {
		if !useTLS {
			return ErrorInvalidSkipVerify
		}

		skipVerify = true
		verbosePrint("certificate verification is going to be skipped!", cmd)
	}

	conn, conErr := SocketConnect(
		server.Address,
		server.Port,
		useTLS,
		skipVerify,
	)
	if conErr != nil {
		return conErr
	}

	err := WaitConnect(cmd, conn, server)
	if err != nil {
		return err
	}

	cmd.Data.Conn = conn

	if cmd.Static.Verbose {
		cmd.Output("Listening for incoming packets...", INFO)
	}

	return nil
}

// Registers a user to a server and also adds it to the client database.
func REG(ctx context.Context, cmd Command, username, pass string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
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
		return ErrorUserExists
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("generating RSA key pair...", cmd)
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
	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr != nil {
		return hashErr
	}

	// Assembles the REG packet
	id := cmd.Data.NextID()
	verbosePrint("performing registration...", cmd)
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(
		spec.REG, id,
		spec.EmptyInfo, pctArgs...,
	)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, err := db.EncryptData([]byte(pass), prvKeyPEM)
	if err != nil {
		return err
	}

	// Creates the user
	_, insertErr := db.AddLocalUser(
		cmd.Static.DB,
		string(username),
		string(hashPass),
		string(enc),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if insertErr != nil {
		return insertErr
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return nil
}

// Deregisters a user from the server and also removes it locally.
func DEREG(ctx context.Context, cmd Command, username, pass string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return existsErr
	}
	if !found {
		return ErrorUserNotFound
	}

	// Verifies password
	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return localUserErr
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return ErrorWrongCredentials
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.DEREG, id, spec.EmptyInfo)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	dbErr := db.DeleteLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dbErr != nil {
		return dbErr
	}

	cmd.Data.LocalUser = nil

	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Output(fmt.Sprintf("user %s deregistered correctly", username), RESULT)
	return nil
}

// Logs a user to a server, also performs the verification.
func LOGIN(ctx context.Context, cmd Command, username, pass string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if cmd.Data.IsLoggedIn() {
		return ErrorAlreadyLoggedIn
	}

	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return existsErr
	}
	if !found {
		return ErrorUserNotFound
	}

	// Verifies password
	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return localUserErr
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return ErrorWrongCredentials
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, err := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if err != nil {
		return err
	}
	localUser.PrvKey = string(dec)

	getPerms := func() {
		perms, err := GetPermissions(ctx, cmd, localUser.User.Username)
		if err == nil {
			str := fmt.Sprintf(
				"Logged in with permission level %d",
				perms,
			)
			cmd.Output(str, INFO)
		}
	}

	// Try to login with a reusable token
	_, validToken := cmd.Data.GetToken()
	if cmd.Data.Server.TLS && validToken {
		err := tokenLogin(ctx, cmd, username)
		if err == nil {
			str := fmt.Sprintf(
				"logged in using a reusable token!\nWelcome %s",
				username,
			)
			cmd.Output(str, RESULT)

			cmd.Data.LocalUser = &localUser
			getPerms()
			return nil
		}

		cmd.Output(err.Error(), ERROR)
		cmd.Output("token verification failed, trying normal login", ERROR)
	}

	// Sends a LOGIN packet with the username as an argument
	verbosePrint("performing login...", cmd)
	id1 := cmd.Data.NextID()
	loginPct, loginPctErr := spec.NewPacket(
		spec.LOGIN, id1,
		spec.EmptyInfo, []byte(username),
	)
	if loginPctErr != nil {
		return loginPctErr
	}

	packetPrint(loginPct, cmd)

	// Sends the packet
	_, loginWErr := cmd.Data.Conn.Write(loginPct)
	if loginWErr != nil {
		return loginWErr
	}

	verbosePrint("awaiting response...", cmd)
	loginReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id1, spec.VERIF, spec.ERR),
	)
	if err != nil {
		return err
	}

	if loginReply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(loginReply.HD.Info)
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
	verbosePrint("performing verification...", cmd)
	id2 := cmd.Data.NextID()
	verifPct, verifPctErr := spec.NewPacket(
		spec.VERIF, id2,
		spec.EmptyInfo,
		[]byte(username), decrypted,
	)
	if verifPctErr != nil {
		return verifPctErr
	}

	packetPrint(verifPct, cmd)

	// Sends the packet
	_, verifWErr := cmd.Data.Conn.Write(verifPct)
	if verifWErr != nil {
		return verifWErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	verifReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id2, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if verifReply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(verifReply.HD.Info)
	}
	verbosePrint("verification successful", cmd)
	// Assigns the logged in user to Data
	cmd.Data.LocalUser = &localUser

	cmd.Output("login successful!", RESULT)
	cmd.Output(fmt.Sprintf("Welcome, %s", username), INFO)
	getPerms()

	if cmd.Data.Server.TLS {
		cmd.Data.SetToken(string(decrypted))
	}

	return nil
}

// Logs out a user from a server.
func LOGOUT(ctx context.Context, cmd Command) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}
	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.LOGOUT, id, spec.EmptyInfo)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	// Sends the packet
	_, pctWErr := cmd.Data.Conn.Write(pct)
	if pctWErr != nil {
		return pctWErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	// Empties the user value in Data
	cmd.Data.LocalUser = nil

	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Output("logged out", RESULT)
	return nil
}

// Disconnects a client from a server.
func DISCN(cmd Command) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	err := cmd.Data.Conn.Close()
	if err != nil {
		return err
	}

	// Closes the client session
	cmd.Data.Conn = nil
	cmd.Data.LocalUser = nil
	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Data.Waitlist.Clear()
	cmd.Output("sucessfully disconnected from the server", RESULT)

	return nil
}

// Sends a message to a user with the current time stamp and stores it in the database.
func MSG(ctx context.Context, cmd Command, username, message string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	// Stores the message before encrypting to store it in the database
	plainMessage := make([]byte, len(message))
	copy(plainMessage, message)

	found, existsErr := db.ExternalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return existsErr
	}
	if !found {
		return ErrorUserNotFound
	}
	// Retrieves the public key in PEM format to encrypt the message
	externalUser, externalUserErr := db.GetExternalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if externalUserErr != nil {
		return externalUserErr
	}
	pubKey, pemErr := spec.PEMToPubkey([]byte(externalUser.PubKey))
	if pemErr != nil {
		return pemErr
	}
	// Encrypts the text
	encrypted, encryptErr := spec.EncryptText([]byte(message), pubKey)
	if encryptErr != nil {
		return encryptErr
	}

	// Generates the packet, using the current UNIX timestamp
	stamp := time.Now().Round(time.Second)
	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(
		spec.MSG, id,
		spec.EmptyInfo,
		[]byte(username),
		spec.UnixStampToBytes(stamp),
		encrypted,
	)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("message sent correctly", RESULT)
	src, srcErr := db.GetUser(
		cmd.Static.DB,
		cmd.Data.LocalUser.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if srcErr != nil {
		return srcErr
	}

	dst, dstErr := db.GetUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dstErr != nil {
		return dstErr
	}

	_, storeErr := db.StoreMessage(
		cmd.Static.DB,
		src.Username,
		dst.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
		string(plainMessage),
		stamp,
	)
	if storeErr != nil {
		return storeErr
	}

	return nil
}

// Asks the server to retrieve all messages while the user was offline.
// This function is not responsible for receiving the messages, only request them.
func RECIV(ctx context.Context, cmd Command) error {
	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.RECIV, id, spec.EmptyInfo)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("messages queried correctly", RESULT)
	return nil
}

// Requests a list of users depending on the type specified, which may or not
// require an active connection.
// Returns a the received usernames in an array if the request was correct.
func USRS(ctx context.Context, cmd Command, usrsType USRSType) ([][]byte, error) {
	// We check for local listing
	switch usrsType {
	case LOCAL_ALL:
		users, err := printAllLocalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	case REQUESTED:
		users, err := printExternalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	case LOCAL_SERVER:
		users, err := printServerLocalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	}

	// We send the usrs petition to the server now

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.USRS, id, byte(usrsType))
	if pctErr != nil {
		return nil, pctErr
	}

	packetPrint(pct, cmd)

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.USRS, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	optionString := "unknown"
	switch usrsType {
	case ALL:
		optionString = "all"
	case ONLINE:
		optionString = "online"
	case ALLPERMS:
		optionString = "all with permissions"
	case ONLINEPERMS:
		optionString = "online with permissions"
	}

	cmd.Output(fmt.Sprintf("%s users:", optionString), USRSRESPONSE)
	cmd.Output(string(reply.Args[0]), USRSRESPONSE)
	split := bytes.Split(reply.Args[0], []byte("\n"))

	return split, nil
}

// Requests the information of an external user to add it to the client database.
// Returns the arguments of a REQ as by specification.
func REQ(ctx context.Context, cmd Command, username string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotConnected
	}

	if username == cmd.Data.LocalUser.User.Username {
		return nil, ErrorRequestToSelf
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(
		spec.REQ, id,
		spec.EmptyInfo, []byte(username),
	)
	if pctErr != nil {
		return nil, pctErr
	}

	packetPrint(pct, cmd)

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.REQ, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	_, dbErr := db.AddExternalUser(
		cmd.Static.DB,
		string(reply.Args[0]),
		string(reply.Args[1]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dbErr != nil {
		return nil, dbErr
	}

	cmd.Output(fmt.Sprintf("external user %s successfully added to the database", username), RESULT)
	return reply.Args, nil
}

// Sends an ADMIN packet that performs an specific ADMIN operation.
func ADMIN(ctx context.Context, cmd Command, op string, args ...[]byte) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	admin, ok := adminList[op]
	if !ok {
		return ErrorInvalidAdminOperation
	}

	min := spec.AdminArgs(admin)
	if len(args) < int(min) {
		return ErrorInsuficientArgs
	}

	arr := make([][]byte, 0, len(args)-1)

	switch admin {
	case spec.AdminShutdown:
		offset, err := strconv.Atoi(string(args[0]))
		if err != nil {
			return err
		}

		shutdown := time.Now().Add(
			time.Duration(offset) * time.Minute,
		)
		unix := spec.UnixStampToBytes(shutdown)

		arr = append(arr, unix)
	case spec.AdminDeregister:
		arr = append(arr, args[0])
	case spec.AdminDisconnect:
		arr = append(arr, args[0])
	case spec.AdminChangePerms:
		num, err := strconv.Atoi(string(args[1]))
		if err != nil {
			return err
		}

		perms := spec.PermissionToBytes(uint(num))

		arr = append(arr, args[0])
		arr = append(arr, perms)
	case spec.AdminMotd:
		motd := bytes.Join(args, []byte(" "))
		arr = append(arr, motd)
	case spec.AdminBroadcast:
		message := bytes.Join(args, []byte(" "))
		arr = append(arr, message)
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.ADMIN, id, uint8(admin), arr...)
	if pctErr != nil {
		return pctErr
	}

	packetPrint(pct, cmd)

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return wErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return err
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output(
		fmt.Sprintf(
			"admin operation %s sent successfully", op,
		), RESULT,
	)
	return nil
}

// Subscribes to a specific hook to the server.
func SUB(ctx context.Context, cmd Command, name string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	hook, ok := hooksList[name]
	if !ok {
		return ErrorUnknownHookOption
	}

	str := fmt.Sprintf("subscribing to event %s...", name)
	verbosePrint(str, cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.SUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return hookPctErr
	}

	packetPrint(hookPct, cmd)

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return hookWErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, replyErr := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if replyErr != nil {
		return replyErr
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("succesfully subscribed!", RESULT)
	return nil
}

// Unsubscribes from a specific hook on the server.
func UNSUB(ctx context.Context, cmd Command, name string) error {
	if !cmd.Data.IsConnected() {
		return ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return ErrorNotLoggedIn
	}

	hook, ok := hooksList[name]
	if !ok {
		return ErrorUnknownHookOption
	}

	str := fmt.Sprintf("unsubscribing to event %s...", name)
	verbosePrint(str, cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.UNSUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return hookPctErr
	}

	packetPrint(hookPct, cmd)

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return hookWErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, replyErr := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if replyErr != nil {
		return replyErr
	}

	if reply.HD.Op == spec.ERR {
		return spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("succesfully unsubscribed!", RESULT)
	return nil
}
