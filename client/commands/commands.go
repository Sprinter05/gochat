package commands

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
	"gorm.io/gorm"
)

// TODO: PENDING and packet buffer
// TODO: cache requested users in memory
// TODO: USERINFO command
// TODO: HELP
// TODO: "/" for commands. If no "/" send message instead
// TODO: More advanced verbose options
// TODO: GETSERVER command

// Struct that contains all the data required for the shell to function.
// Commands may alter the data if necessary.
type Data struct {
	// TODO: Thread safe??
	ClientCon spec.Connection
	Server    db.Server
	User      db.LocalUser
	Waitlist  models.Waitlist[spec.Command]
}

// Separated struct that eases interaction with the terminal UI
type StaticData struct {
	Verbose bool
	DB      *gorm.DB
}

type Command struct {
	Output func(text string, outputType OutputType) // Custom output-printing function
	Static *StaticData
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

// Represents the type of a command output.
// This eases specific output printing actions.
type OutputType uint

const (
	INTERMEDIATE OutputType = iota // Intermediate status messages
	PACKET                         // Packet information messages
	PROMPT                         // Prompt message when input is asked
	RESULT                         // Messages that show the result of a command
	ERROR                          // Error messages that may be printed additionaly in error cases
	INFO                           // Message that representes generic info not asocciated to a command
	USRS                           // Specific for user printing
)

// Possible command errors.
var (
	ErrorInsuficientArgs   error = fmt.Errorf("not enough arguments")
	ErrorNotConnected      error = fmt.Errorf("not connected to a server")
	ErrorAlreadyConnected  error = fmt.Errorf("already connected to a server")
	ErrorNotLoggedIn       error = fmt.Errorf("you are not logged in")
	ErrorAlreadyLoggedIn   error = fmt.Errorf("you are already logged in")
	ErrorWrongCredentials  error = fmt.Errorf("wrong credentials")
	ErrorUnknownUSRSOption error = fmt.Errorf("unknown option; valid options are online, all or local")
	ErrorUsernameEmpty     error = fmt.Errorf("username cannot be empty")
	ErrorUserExists        error = fmt.Errorf("user exists")
	ErrorPasswordsNotMatch error = fmt.Errorf("passwords do not match")
	ErrorUserNotFound      error = fmt.Errorf("user not found")
)

// Map that contains every shell command with its respective execution functions.
var clientCmds = map[string]func(cmd Command, args ...[]byte) ReplyData{
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
func FetchClientCmd(op string, cmd Command) func(cmd Command, args ...[]byte) ReplyData {
	v, ok := clientCmds[strings.ToUpper(op)]
	if !ok {
		cmd.Output(fmt.Sprintf("%s: command not found", op), ERROR)
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
func Conn(cmd Command, args ...[]byte) ReplyData {
	if cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorAlreadyConnected}
	}
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	address := string(args[0])
	port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
	if parseErr != nil {
		return ReplyData{Error: parseErr}
	}

	con, conErr := Connect(address, uint16(port))
	if conErr != nil {
		return ReplyData{Error: conErr}
	}

	server, dbErr := db.SaveServer(cmd.Static.DB, address, uint16(port), "Default")
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}

	cmd.Data.ClientCon.Conn = con
	cmd.Data.Server = server
	err := ConnectionStart(cmd)
	if err != nil {
		return ReplyData{Error: err}
	}

	cmd.Output("listening for incoming packets...", INFO)
	go Listen(&cmd)
	return ReplyData{}
}

// Disconnects a client from a gochat server.
//
// Arguments: none
//
// Returns a zero value ReplyData if the disconnection was successful.
func Discn(cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	err := cmd.Data.ClientCon.Conn.Close()
	if err != nil {
		return ReplyData{Error: err}
	}
	cmd.Data.ClientCon.Conn = nil
	// Closes the shell client session
	cmd.Data.User = db.LocalUser{}
	cmd.Output("sucessfully disconnected from the server", RESULT)
	return ReplyData{}
}

// Prints the gochat version used by the client
func Ver(data Command, args ...[]byte) ReplyData {
	data.Output(fmt.Sprintf("gochat version %d", spec.ProtocolVersion), RESULT)
	return ReplyData{}
}

// Switches on/off the verbose mode.
//
// Arguments: none
//
// Returns a zero value ReplyData.
func Verbose(cmd Command, args ...[]byte) ReplyData {
	cmd.Static.Verbose = !cmd.Static.Verbose
	if cmd.Static.Verbose {
		cmd.Output("verbose mode on", RESULT)
	} else {
		cmd.Output("verbose mode off", RESULT)
	}
	return ReplyData{}
}

// Requests the information of an external user to add it to the client database.
//
// Arguments: <username to be requested>
//
// Returns a ReplyData containing the reply REQ arguments.
func Req(cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !cmd.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.REQ, 1, spec.EmptyInfo, args...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, wErr := cmd.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply := cmd.Data.Waitlist.Get(Find(1, spec.REQ, spec.ERR))

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	_, dbErr := db.AddExternalUser(cmd.Static.DB, string(reply.Args[0]), string(reply.Args[1]), cmd.Data.Server.ServerID)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}
	cmd.Output(fmt.Sprintf("user %s successfully added to the database", args[0]), RESULT)
	return ReplyData{Arguments: reply.Args}
}

// Registers a user to a server and also adds it to the client database.
// A prompt will get the user input if the user and password is not specified.
//
// Arguments: [user] [password]
//
// Returns a zero value ReplyData if an OK packet is received after the sent REG packet.
func Reg(cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
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
		cmd.Output("username: ", PROMPT)
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

		exists, existsErr := db.LocalUserExists(cmd.Static.DB, string(username))
		if existsErr != nil {
			return ReplyData{Error: existsErr}
		}
		if exists {
			return ReplyData{Error: ErrorUserExists}
		}

		// Gets the password
		cmd.Output("password: ", PROMPT)
		var pass1Err error
		pass1, pass1Err = term.ReadPassword(0)
		if pass1Err != nil {
			cmd.Output("", PROMPT)
			return ReplyData{Error: pass1Err}
		}
		cmd.Output("\n", PROMPT)

		cmd.Output("repeat password: ", PROMPT)
		pass2, pass2Err := term.ReadPassword(0)
		if pass2Err != nil {
			cmd.Output("\n", PROMPT)
			return ReplyData{Error: pass2Err}
		}
		cmd.Output("\n", PROMPT)

		if string(pass1) != string(pass2) {
			return ReplyData{Error: ErrorPasswordsNotMatch}
		}
	} else {
		username = args[0]
		pass1 = args[1]
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("generating RSA key pair...", cmd)
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
	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass1, 12)
	if hashErr != nil {
		return ReplyData{Error: hashErr}
	}

	verbosePrint("performing registration...", cmd)
	// Assembles the REG packet
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(spec.REG, 1, spec.EmptyInfo, pctArgs...)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply := cmd.Data.Waitlist.Get(Find(1, spec.OK, spec.ERR))

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Creates the user
	_, insertErr := db.AddLocalUser(cmd.Static.DB, string(username), string(hashPass), string(prvKeyPEM), cmd.Data.Server.ServerID)
	if insertErr != nil {
		return ReplyData{Error: insertErr}
	}
	cmd.Output(fmt.Sprintf("user %s successfully added to the database", username), RESULT)
	return ReplyData{}
}

// Logs a user to a server. If only the username
// is given, the command will ask for the password.
//
// Arguments: <username> [password]
//
// Returns a zero value ReplyData if an OK packet
// is received after the sent VERIF packet.
func Login(cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if cmd.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorAlreadyLoggedIn}
	}
	username := string(args[0])
	found, existsErr := db.LocalUserExists(cmd.Static.DB, username)
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}

	var pass []byte
	var passErr error

	if len(args) == 1 {
		// Asks for password
		cmd.Output(fmt.Sprintf("%s's password: ", username), PROMPT)
		pass, passErr = term.ReadPassword(0)
		if passErr != nil {
			cmd.Output("\n", PROMPT)
			return ReplyData{Error: passErr}
		}
		cmd.Output("\n", PROMPT)
	} else {
		pass = args[1]
	}

	// Verifies password
	localUser, localUserErr := db.GetLocalUser(cmd.Static.DB, username, cmd.Data.Server.ServerID)
	if localUserErr != nil {
		return ReplyData{Error: localUserErr}
	}
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ReplyData{Error: ErrorWrongCredentials}
	}

	verbosePrint("password correct, performing login...", cmd)
	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	loginPct, loginPctErr := spec.NewPacket(spec.LOGIN, 1, spec.EmptyInfo, args[0])
	if loginPctErr != nil {
		return ReplyData{Error: loginPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(loginPct, cmd)
	}

	// Sends the packet
	_, loginWErr := cmd.Data.ClientCon.Conn.Write(loginPct)
	if loginWErr != nil {
		return ReplyData{Error: loginWErr}
	}

	verbosePrint("awaiting response...", cmd)
	loginReply := cmd.Data.Waitlist.Get(Find(1, spec.VERIF, spec.ERR))

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

	verbosePrint("performing verification...", cmd)
	// Sends a reply to the VERIF packet
	verifPct, verifPctErr := spec.NewPacket(spec.VERIF, 1, spec.EmptyInfo, []byte(username), decrypted)
	if verifPctErr != nil {
		return ReplyData{Error: verifPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(verifPct, cmd)
	}

	// Sends the packet
	_, verifWErr := cmd.Data.ClientCon.Conn.Write(verifPct)
	if verifWErr != nil {
		return ReplyData{Error: verifWErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	verifReply := cmd.Data.Waitlist.Get(Find(1, spec.OK, spec.ERR))

	if verifReply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(verifReply.HD.Info)}
	}
	verbosePrint("verification successful", cmd)
	// Assigns the logged in user to Data
	cmd.Data.User = localUser

	cmd.Output(fmt.Sprintf("login successful. Welcome, %s", username), RESULT)
	return ReplyData{Arguments: verifReply.Args}
}

// Logs out a user from a server.
//
// Arguments: none
//
// Returns a zero value ReplyData if an OK packet is received after the sent LOGOUT packet.
func Logout(cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !cmd.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	pct, pctErr := spec.NewPacket(spec.LOGOUT, 1, spec.EmptyInfo)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, pctWErr := cmd.Data.ClientCon.Conn.Write(pct)
	if pctWErr != nil {
		return ReplyData{Error: pctWErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply := cmd.Data.Waitlist.Get(Find(1, spec.OK, spec.ERR))

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Empties the user value in Data
	cmd.Data.User = db.LocalUser{}

	cmd.Output("logged out", RESULT)
	return ReplyData{}
}

// Requests a list of either "online" or "all" registered users and prints it. If "local"
// is used as an argument, the local users will be printed insteads and no server requests
// will be performed.
//
// Arguments: <online/all/local>
//
// Returns a zero value ReplyData if an OK packet is received after the sent VERIF packet.
func Usrs(cmd Command, args ...[]byte) ReplyData {
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !cmd.Data.IsConnected() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !cmd.Data.IsUserLoggedIn() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	var option byte
	switch string(args[0]) {
	case "online":
		option = 0x01
	case "all":
		option = 0x00
	case "local":
		users, err := printLocalUsers(cmd)
		if err != nil {
			return ReplyData{Error: err}
		}
		return ReplyData{Arguments: users}
	default:
		return ReplyData{Error: ErrorUnknownUSRSOption}
	}

	pct, pctErr := spec.NewPacket(spec.USRS, 1, option)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply := cmd.Data.Waitlist.Get(Find(1, spec.USRS, spec.ERR))

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output(fmt.Sprintf("%s users:", args[0]), USRS)
	cmd.Output(string(reply.Args[0]), USRS)
	split := bytes.Split(reply.Args[0], []byte("\n"))
	return ReplyData{Arguments: split}
}

// Sends a message to a user with the current time stamp and stores it in the database.
//
// Arguments: <dest. username> <unencyrpted text message>
//
// Returns a zero value ReplyData if an OK packet is received after the sent MSG packet
func Msg(cmd Command, args ...[]byte) ReplyData {
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !cmd.Data.IsUserLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}
	// Stores the message before encrypting to store it in the database
	plainMessage := make([]byte, len(args[1]))
	copy(plainMessage, args[1])

	found, existsErr := db.ExternalUserExists(cmd.Static.DB, string(args[0]))
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}
	// Retrieves the public key in PEM format to encrypt the message
	externalUser, externalUserErr := db.GetExternalUser(cmd.Static.DB, string(args[0]), cmd.Data.Server.ServerID)
	if externalUserErr != nil {
		return ReplyData{Error: externalUserErr}
	}
	pubKey, pemErr := spec.PEMToPubkey([]byte(externalUser.PubKey))
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

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.ClientCon.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply := cmd.Data.Waitlist.Get(Find(1, spec.OK, spec.ERR))

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output("message sent correctly", RESULT)
	src, srcErr := db.GetUser(cmd.Static.DB, cmd.Data.User.User.Username, cmd.Data.Server.ServerID)
	if srcErr != nil {
		return ReplyData{Error: srcErr}
	}
	dst, dstErr := db.GetUser(cmd.Static.DB, string(args[0]), cmd.Data.Server.ServerID)
	if dstErr != nil {
		return ReplyData{Error: dstErr}
	}
	_, storeErr := db.StoreMessage(cmd.Static.DB, src, dst, string(plainMessage), stamp)
	if storeErr != nil {
		return ReplyData{Error: storeErr}
	}
	return ReplyData{}
}

// Prints out all local users and returns an array with its usernames.
func printLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetAllLocalUsernames(cmd.Static.DB)
	if err != nil {
		return [][]byte{}, err
	}
	users := make([][]byte, 0, len(localUsers))
	cmd.Output("local users:", USRS)
	for _, v := range localUsers {
		users = append(users, []byte(v))
		cmd.Output(v, USRS)
	}
	return users, nil
}

// Prints a packet.
func packetPrint(pct []byte, cmd Command) {
	// TODO: remove the Print and print the string obtained
	pctCmd := spec.ParsePacket(pct)
	str := fmt.Sprintf(
		"Client packet to be sent:\n%s",
		pctCmd.Contents(),
	)
	cmd.Output(str, PACKET)
}

// Prints text if the verbose mode is on.
func verbosePrint(text string, args Command) {
	if args.Static.Verbose {
		args.Output(text, INTERMEDIATE)
	}
}

// Returns a function that returns true if the received command fulfills
// the given conditions in the arguments (ID and operations).
// This is used to dinamically create functions that retrieve commands
// from the waitlist with waitlist.Get()
func Find(id spec.ID, ops ...spec.Action) func(cmd spec.Command) bool {
	findFunc := func(cmd spec.Command) bool {
		if cmd.HD.ID == id {
			if slices.Contains(ops, cmd.HD.Op) {
				return true
			}
		}
		return false
	}
	return findFunc
}
