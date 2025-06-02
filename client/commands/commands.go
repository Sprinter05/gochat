package commands

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
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
// TODO: turn duplicated code (import/reg) to aux functions
// TODO: move ask password function to shell package

/* STRUCTS */

// Struct that contains all the data required for the shell to function.
// Commands may alter the data if necessary.
type Data struct {
	// TODO: Thread safe??
	Conn     net.Conn
	Server   *db.Server
	User     *db.LocalUser
	Waitlist models.Waitlist[spec.Command]
	Next     spec.ID
	Logout   context.CancelFunc
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

// Contains data received from the reply of a command.
type ReplyData struct {
	Arguments [][]byte
	Error     error
}

// Represents the type of a command output.
// This eases specific output printing actions.
type OutputType uint

/* DATA FUNCTIONS */

func NewEmptyData() *Data {
	return &Data{
		Waitlist: DefaultWaitlist(),
		Next:     spec.NullID + 1,
		Logout:   func() {},
	}
}

func (data *Data) NextID() spec.ID {
	data.Next = (data.Next + 1) % spec.MaxID
	if data.Next == spec.NullID {
		data.Next += 1
	}
	return data.Next
}

func (data *Data) IsLoggedIn() bool {
	return data.User != nil && data.User.User.Username != "" && data.IsConnected()
}

func (data *Data) IsConnected() bool {
	return data.Conn != nil
}

/* OUTPUT TYPES */

const (
	INTERMEDIATE OutputType = iota // Intermediate status messages
	PACKET                         // Packet information messages
	PROMPT                         // Prompt message when input is asked
	RESULT                         // Messages that show the result of a command
	ERROR                          // Error messages that may be printed additionaly in error cases
	INFO                           // Message that representes generic info not asocciated to a command
	USRS                           // Specific for user printing
)

/* ERRORS */

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
	ErrorUserExists        error = fmt.Errorf("local user exists")
	ErrorPasswordsNotMatch error = fmt.Errorf("passwords do not match")
	ErrorUserNotFound      error = fmt.Errorf("local user not found")
	ErrorUnknownTLSOption  error = fmt.Errorf("unknown option; valid options are on or off")
	ErrorOfflineRequired   error = fmt.Errorf("you must be offline")
	ErrorInvalidSkipVerify error = fmt.Errorf("cannot skip verification on a non-TLS endpoint")
	ErrorRequestToSelf     error = fmt.Errorf("cannot request yourself")
	ErrorUnknownHookOption error = fmt.Errorf("invalid hook provided")
)

/* LOOKUP TABLE */

type cmdFunc func(context.Context, Command, ...[]byte) ReplyData

// Map that contains every shell command with its respective execution functions.
var clientCmds = map[string]cmdFunc{
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
	"RECIV":   Reciv,
	"TLS":     TLS,
	"IMPORT":  Import,
	"EXPORT":  Export,
	"SUB":     Sub,
	"UNSUB":   Unsub,
}

// List of hooks and their names.
var hooksList = map[string]spec.Hook{
	"all":                spec.HookAllHooks,
	"new_login":          spec.HookNewLogin,
	"new_logout":         spec.HookNewLogout,
	"duplicated_session": spec.HookDuplicateSession,
	"permissions_change": spec.HookPermsChange,
}

// Given a string containing a command name, returns its execution function.
func FetchClientCmd(op string, cmd Command) cmdFunc {
	v, ok := clientCmds[strings.ToUpper(op)]
	if !ok {
		cmd.Output(
			fmt.Sprintf("%s: command not found", op),
			ERROR,
		)

		return nil
	}
	return v
}

/* CLIENT COMMANDS */

// Subscribes to a specific hook to the server
//
// Arguments: <hook>
//
// Returns a zero value ReplyData if successful
func Sub(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if !cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	hook, ok := hooksList[string(args[0])]
	if !ok {
		return ReplyData{Error: ErrorUnknownHookOption}
	}

	verbosePrint("subscribing to event...", cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.SUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return ReplyData{Error: hookPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(hookPct, cmd)
	}

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return ReplyData{Error: hookWErr}
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output("succesfully subscribed!", RESULT)
	return ReplyData{}
}

// Unsubscribes from a specific hook to the server
//
// Arguments: <hook>
//
// Returns a zero value ReplyData if successful
func Unsub(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if !cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	hook, ok := hooksList[string(args[0])]
	if !ok {
		return ReplyData{Error: ErrorUnknownHookOption}
	}

	verbosePrint("unsubscribing to event...", cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.UNSUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return ReplyData{Error: hookPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(hookPct, cmd)
	}

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return ReplyData{Error: hookWErr}
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output("succesfully unsubscribed!", RESULT)
	return ReplyData{}
}

// Imports a private RSA key for a new local user
// from the specified directory using the spec PEM format
// and then performs a registration on the server.
//
// Arguments: <username> <path> [password]
//
// Returns a zero value ReplyData if successful
func Import(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	username := string(args[0])
	path := string(args[1])

	var pass []byte

	if len(args) < 3 {
		// todo shell ask for password
	} else {
		pass = args[2]
	}

	verbosePrint("reading private key...", cmd)
	buf, err := os.ReadFile(path)
	if err != nil {
		return ReplyData{Error: err}
	}

	key, chk := spec.PEMToPrivkey(buf)
	if chk != nil {
		return ReplyData{Error: chk}
	}

	pub, err := spec.PubkeytoPEM(&key.PublicKey)
	if err != nil {
		return ReplyData{Error: err}
	}

	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword(pass, 12)
	if hashErr != nil {
		return ReplyData{Error: hashErr}
	}

	id := cmd.Data.NextID()
	verbosePrint("performing registration...", cmd)
	pctArgs := [][]byte{[]byte(username), pub}
	pct, pctErr := spec.NewPacket(
		spec.REG, id,
		spec.EmptyInfo, pctArgs...,
	)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, err := db.EncryptData([]byte(pass), buf)
	if err != nil {
		return ReplyData{Error: err}
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
		return ReplyData{Error: insertErr}
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return ReplyData{}
}

// Exports a local user as a private RSA key
// in the current directory using the spec PEM format
//
// Arguments: <username> [password]
//
// Returns a zero value ReplyData if successful
func Export(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	username := string(args[0])
	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}

	var pass []byte

	if len(args) < 2 {
		// todo shell ask for password
	} else {
		pass = args[1]
	}

	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return ReplyData{Error: localUserErr}
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ReplyData{Error: ErrorWrongCredentials}
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, err := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if err != nil {
		return ReplyData{Error: err}
	}
	localUser.PrvKey = string(dec)

	file := username + ".priv"
	f, err := os.Create(file)
	if err != nil {
		return ReplyData{Error: err}
	}
	defer f.Close()

	_, writeErr := f.Write([]byte(localUser.PrvKey))
	if writeErr != nil {
		return ReplyData{Error: writeErr}
	}

	str := fmt.Sprintf(
		"file succesfully written to %s", f.Name(),
	)
	cmd.Output(str, RESULT)
	return ReplyData{}
}

// Changes the state of a TLS server
//
// Arguments: <on/off>
//
// Returns a zero value ReplyData if the argument is correct
func TLS(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorOfflineRequired}
	}

	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	if string(args[0]) == "on" {
		cmd.Data.Server.TLS = true
		err := db.ChangeServerTLS(
			cmd.Static.DB,
			cmd.Data.Server.Address,
			cmd.Data.Server.Port,
			true,
		)

		if err != nil {
			return ReplyData{Error: err}
		}

		return ReplyData{}
	} else if string(args[0]) == "off" {
		cmd.Data.Server.TLS = false
		err := db.ChangeServerTLS(
			cmd.Static.DB,
			cmd.Data.Server.Address,
			cmd.Data.Server.Port,
			false,
		)

		if err != nil {
			return ReplyData{Error: err}
		}

		return ReplyData{}
	}

	return ReplyData{Error: ErrorUnknownTLSOption}
}

// Starts a connection with a server. If noverify is set,
// in case of TLS connections, certificate origins wont be checked
//
// Arguments: <server address> <server port> [-noverify]
//
// Returns a zero value ReplyData if the connection was successful.
// This command does not spawn a listening thread nor allocates a waitlist.
func Conn(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorAlreadyConnected}
	}

	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	port, parseErr := strconv.ParseUint(string(args[1]), 10, 16)
	if parseErr != nil {
		return ReplyData{Error: parseErr}
	}

	useTLS := cmd.Data.Server.TLS
	skipVerify := false

	if len(args) >= 3 && string(args[2]) == "-noverify" {
		if !useTLS {
			return ReplyData{Error: ErrorInvalidSkipVerify}
		}

		skipVerify = true
		verbosePrint("certificate verification is going to be skipped!", cmd)
	}

	con, conErr := Connect(
		string(args[0]),
		uint16(port),
		useTLS,
		skipVerify,
	)
	if conErr != nil {
		return ReplyData{Error: conErr}
	}

	// server, dbErr := db.GetServer(cmd.Static.DB, string(args[0]), uint16(port))
	// if dbErr != nil {
	// 	return ReplyData{Error: dbErr}
	// }

	cmd.Data.Conn = con
	err := ConnectionStart(cmd)

	if err != nil {
		return ReplyData{Error: err}
	}

	cmd.Output("listening for incoming packets...", INFO)

	return ReplyData{}
}

// Disconnects a client from a gochat server.
//
// Arguments: none
//
// Returns a zero value ReplyData if the disconnection was successful.
func Discn(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	err := cmd.Data.Conn.Close()
	if err != nil {
		return ReplyData{Error: err}
	}

	// Closes the shell client session
	cmd.Data.Conn = nil
	cmd.Data.User = nil
	cmd.Data.Waitlist.Clear()
	cmd.Output("sucessfully disconnected from the server", RESULT)

	return ReplyData{}
}

// Prints the gochat version used by the client
func Ver(ctx context.Context, data Command, args ...[]byte) ReplyData {
	data.Output(
		fmt.Sprintf(
			"gochat version %d",
			spec.ProtocolVersion,
		), RESULT,
	)

	return ReplyData{}
}

// Switches on/off the verbose mode.
//
// Arguments: none
//
// Returns a zero value ReplyData.
func Verbose(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
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
func Req(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	if !cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	if string(args[0]) == cmd.Data.User.User.Username {
		return ReplyData{Error: ErrorRequestToSelf}
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(
		spec.REQ, id,
		spec.EmptyInfo, args...,
	)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.REQ, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	_, dbErr := db.AddExternalUser(
		cmd.Static.DB,
		string(reply.Args[0]),
		string(reply.Args[1]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dbErr != nil {
		return ReplyData{Error: dbErr}
	}

	cmd.Output(fmt.Sprintf("external user %s successfully added to the database", args[0]), RESULT)
	return ReplyData{Arguments: reply.Args}
}

// Registers a user to a server and also adds it to the client database.
// A prompt will get the user input if the user and password is not specified.
//
// Arguments: [user] [password]
//
// Returns a zero value ReplyData if an OK packet is received after the sent REG packet.
func Reg(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if len(args) == 1 {
		return ReplyData{Error: spec.ErrorArguments}
	}

	var username []byte
	var pass1 []byte

	if len(args) < 2 {
		// todo move to shell package
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

	exists, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		string(username),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if exists {
		return ReplyData{Error: ErrorUserExists}
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

	// Assembles the REG packet
	id := cmd.Data.NextID()
	verbosePrint("performing registration...", cmd)
	pctArgs := [][]byte{[]byte(username), pubKeyPEM}
	pct, pctErr := spec.NewPacket(
		spec.REG, id,
		spec.EmptyInfo, pctArgs...,
	)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, err := db.EncryptData([]byte(pass1), prvKeyPEM)
	if err != nil {
		return ReplyData{Error: err}
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
		return ReplyData{Error: insertErr}
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return ReplyData{}
}

// Logs a user to a server. If only the username
// is given, the command will ask for the password.
//
// Arguments: <username> [password]
//
// Returns a zero value ReplyData if an OK packet
// is received after the sent VERIF packet.
func Login(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	if cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorAlreadyLoggedIn}
	}

	username := string(args[0])
	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}

	var pass []byte
	var passErr error

	if len(args) < 2 {
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
	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return ReplyData{Error: localUserErr}
	}

	// // In case the foreign key is not auto filled
	// user, userErr := db.GetUser(
	// 	cmd.Static.DB,
	// 	username,
	// 	cmd.Data.Server.Address,
	// 	cmd.Data.Server.Port,
	// )
	// if userErr != nil {
	// 	return ReplyData{Error: userErr}
	// }
	// localUser.User = user

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, pass)
	if cmpErr != nil {
		return ReplyData{Error: ErrorWrongCredentials}
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, err := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if err != nil {
		return ReplyData{Error: err}
	}
	localUser.PrvKey = string(dec)

	// TODO: token
	// Sends a LOGIN packet with the username as an argument
	verbosePrint("performing login...", cmd)
	id1 := cmd.Data.NextID()
	loginPct, loginPctErr := spec.NewPacket(
		spec.LOGIN, id1,
		spec.EmptyInfo, args[0],
	)
	if loginPctErr != nil {
		return ReplyData{Error: loginPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(loginPct, cmd)
	}

	// Sends the packet
	_, loginWErr := cmd.Data.Conn.Write(loginPct)
	if loginWErr != nil {
		return ReplyData{Error: loginWErr}
	}

	verbosePrint("awaiting response...", cmd)
	loginReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id1, spec.VERIF, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
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
	verbosePrint("performing verification...", cmd)
	id2 := cmd.Data.NextID()
	verifPct, verifPctErr := spec.NewPacket(
		spec.VERIF, id2,
		spec.EmptyInfo,
		[]byte(username), decrypted,
	)
	if verifPctErr != nil {
		return ReplyData{Error: verifPctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(verifPct, cmd)
	}

	// Sends the packet
	_, verifWErr := cmd.Data.Conn.Write(verifPct)
	if verifWErr != nil {
		return ReplyData{Error: verifWErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	verifReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id2, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if verifReply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(verifReply.HD.Info)}
	}
	verbosePrint("verification successful", cmd)
	// Assigns the logged in user to Data
	cmd.Data.User = &localUser

	cmd.Output(fmt.Sprintf("login successful. Welcome, %s", username), RESULT)
	return ReplyData{Arguments: verifReply.Args}
}

// Logs out a user from a server.
//
// Arguments: none
//
// Returns a zero value ReplyData if an OK packet is received after the sent LOGOUT packet.
func Logout(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}
	if !cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.LOGOUT, id, spec.EmptyInfo)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, pctWErr := cmd.Data.Conn.Write(pct)
	if pctWErr != nil {
		return ReplyData{Error: pctWErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	// Empties the user value in Data
	cmd.Data.User = nil

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
func Usrs(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if len(args) < 1 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	if !cmd.Data.IsConnected() && !(string(args[0]) == "local") {
		return ReplyData{Error: ErrorNotConnected}
	}

	if !cmd.Data.IsLoggedIn() && !(string(args[0]) == "local") {
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

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.USRS, id, option)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.USRS, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

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
func Msg(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	if len(args) < 2 {
		return ReplyData{Error: ErrorInsuficientArgs}
	}

	if !cmd.Data.IsConnected() {
		return ReplyData{Error: ErrorNotConnected}
	}

	if !cmd.Data.IsLoggedIn() {
		return ReplyData{Error: ErrorNotLoggedIn}
	}

	// Stores the message before encrypting to store it in the database
	plainMessage := make([]byte, len(args[1]))
	copy(plainMessage, args[1])

	found, existsErr := db.ExternalUserExists(
		cmd.Static.DB,
		string(args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return ReplyData{Error: existsErr}
	}
	if !found {
		return ReplyData{Error: ErrorUserNotFound}
	}
	// Retrieves the public key in PEM format to encrypt the message
	externalUser, externalUserErr := db.GetExternalUser(
		cmd.Static.DB,
		string(args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
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

	// Generates the packet, using the current UNIX timestamp
	stamp := time.Now().Round(time.Second)
	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(
		spec.MSG, id,
		spec.EmptyInfo,
		args[0],
		spec.UnixStampToBytes(stamp),
		encrypted,
	)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return ReplyData{Error: wErr}
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output("message sent correctly", RESULT)
	src, srcErr := db.GetUser(
		cmd.Static.DB,
		cmd.Data.User.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if srcErr != nil {
		return ReplyData{Error: srcErr}
	}

	dst, dstErr := db.GetUser(
		cmd.Static.DB,
		string(args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dstErr != nil {
		return ReplyData{Error: dstErr}
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
		return ReplyData{Error: storeErr}
	}

	return ReplyData{}
}

// Sends a RECIV packet to the server. This command listens for an initial ERR/OK
//
// Arguments: none
//
// Returns a zero value ReplyData if the packet is sent successfully
func Reciv(ctx context.Context, cmd Command, args ...[]byte) ReplyData {
	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.RECIV, id, spec.EmptyInfo)
	if pctErr != nil {
		return ReplyData{Error: pctErr}
	}

	_, writeErr := cmd.Data.Conn.Write(pct)
	if writeErr != nil {
		return ReplyData{Error: writeErr}
	}

	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return ReplyData{Error: err}
	}

	if reply.HD.Op == spec.ERR {
		return ReplyData{Error: spec.ErrorCodeToError(reply.HD.Info)}
	}

	cmd.Output("messages queried correctly", RESULT)
	return ReplyData{}
}

/* AUX */

// Performs the necessary operations to store a RECIV
// packet in the database (decryption, REQ (if necessary)
// insert...), then returns the decrypted message
func StoreReciv(ctx context.Context, reciv spec.Command, cmd Command) (Message, error) {
	src, err := db.GetUser(
		cmd.Static.DB,
		string(reciv.Args[0]),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if err != nil {
		// The user most likely has not been found, so a REQ is required
		reply := Req(ctx, cmd, reciv.Args[0])
		if reply.Error != nil {
			return Message{}, reply.Error
		}
	}

	prvKey, pemErr := spec.PEMToPrivkey([]byte(cmd.Data.User.PrvKey))
	if pemErr != nil {
		return Message{}, pemErr
	}

	decrypted, decryptErr := spec.DecryptText(reciv.Args[2], prvKey)
	if decryptErr != nil {
		return Message{}, decryptErr
	}

	stamp, parseErr := spec.BytesToUnixStamp(reciv.Args[1])
	if parseErr != nil {
		return Message{}, parseErr
	}

	_, insertErr := db.StoreMessage(
		cmd.Static.DB,
		src.Username,
		cmd.Data.User.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
		string(decrypted),
		stamp,
	)
	if insertErr != nil {
		return Message{}, insertErr
	}

	return Message{
		Sender:    string(reciv.Args[0]),
		Content:   string(decrypted),
		Timestamp: stamp,
	}, nil
}

/* AUXILIARY FUNCTIONS */

// Prints out all local users and returns an array with its usernames.
func printLocalUsers(cmd Command) ([][]byte, error) {
	localUsers, err := db.GetAllLocalUsernames(
		cmd.Static.DB,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
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

/* WAITLIST FUNCTIONS */

// Returns a function that returns true if the received command fulfills
// the given conditions in the arguments (ID and operations).
// This is used to dinamically create functions that retrieve commands
// from the waitlist with waitlist.Get()
func Find(id spec.ID, ops ...spec.Action) func(cmd spec.Command) bool {
	return func(cmd spec.Command) bool {
		if cmd.HD.ID == id && slices.Contains(ops, cmd.HD.Op) {
			return true
		}

		return false
	}
}

// Returns an appropiate waitlist
func DefaultWaitlist() models.Waitlist[spec.Command] {
	return models.NewWaitlist(0, func(a spec.Command, b spec.Command) int {
		switch {
		case a.HD.ID > b.HD.ID:
			return 1
		case a.HD.ID < b.HD.ID:
			return -1
		default:
			return 0
		}
	})
}
