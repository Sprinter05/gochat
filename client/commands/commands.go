package commands

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	mrand "math/rand/v2"
	"net"
	"os"
	"time"

	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

/* STRUCTS */

// Struct that contains all the data necessary to run a command
// Requires fields may change between commands
// Commands may alter the data if necessary
type Data struct {
	Conn     net.Conn                      // Specifies the connection to the server
	Server   *db.Server                    // Specifies the database server
	User     *db.LocalUser                 // Specifies the logged in user
	Logout   context.CancelFunc            // Specifies the function to call on a logout for context propagation
	Waitlist models.Waitlist[spec.Command] // Stores all commands to be retrieved later
	Next     spec.ID                       // Specifies the next ID that should be used when sending a packet
}

// Static data that should only be assigned
// in specific cases
type StaticData struct {
	Verbose bool     // Whether or not to print detailed information
	DB      *gorm.DB // Connection to the database
}

// Specifies all structs necessary for a command
type Command struct {
	Output OutputFunc  // Custom output-printing function
	Static *StaticData // Static Data (mostly)
	Data   *Data       // Modifiable Data
}

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
	USRS                           // Specific for user printing
	COLOR                          // Special output for shell colors
	PLAIN                          // Output type that should be printed as-is, with no prefix
)

// Represents the function that will be called when outputting info
type OutputFunc func(text string, outputType OutputType)

// Represents the different USRS command types
type USRSType uint

const (
	ALL          USRSType = 0 // Users in the server (as spec)
	ONLINE       USRSType = 1 // Online users in the server (as spec)
	LOCAL_SERVER USRSType = 2 // Registered local users for a server
	LOCAL_ALL    USRSType = 3 // All local users
	REQUESTED    USRSType = 4 // All external users whose public key has been saved
)

/* DATA FUNCTIONS */

// Creates a new empty but initialised struct for Data
func NewEmptyData() Data {
	initial := mrand.IntN(int(spec.MaxID))

	return Data{
		Waitlist: DefaultWaitlist(),
		Next:     spec.ID(initial),
		Logout:   func() {},
	}
}

// Incremenents the next ID to be used and returns it
func (data *Data) NextID() spec.ID {
	data.Next = (data.Next + 1) % spec.MaxID
	if data.Next == spec.NullID {
		data.Next += 1
	}
	return data.Next
}

// Whether the connection is logged in or not
func (data *Data) IsLoggedIn() bool {
	return data.User != nil && data.User.User.Username != "" && data.IsConnected()
}

// Whether the connection is or not established
func (data *Data) IsConnected() bool {
	return data.Conn != nil
}

/* ERRORS */

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
)

/* LOOKUP TABLES */

// List of hooks and their names.
var hooksList = map[string]spec.Hook{
	"all":                spec.HookAllHooks,
	"new_login":          spec.HookNewLogin,
	"new_logout":         spec.HookNewLogout,
	"duplicated_session": spec.HookDuplicateSession,
	"permissions_change": spec.HookPermsChange,
}

// List of all necessary arguments for each admin command.
var adminArgs = map[spec.Admin]uint{
	spec.AdminBroadcast:   1,
	spec.AdminDeregister:  1,
	spec.AdminChangePerms: 2,
	spec.AdminDisconnect:  1,
	spec.AdminShutdown:    1,
	spec.AdminMotd:        1,
}

/* CLIENT COMMANDS */

// Imports a private RSA key for a new local user
// from the specified directory using the specification PEM format.
func Import(cmd Command, username, pass, path string) ([][]byte, error) {
	verbosePrint("reading private key...", cmd)
	buf, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil, readErr
	}

	_, chkErr := spec.PEMToPrivkey(buf)
	if chkErr != nil {
		return nil, chkErr
	}

	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr != nil {
		return nil, hashErr
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, encryptErr := db.EncryptData([]byte(pass), buf)
	if encryptErr != nil {
		return nil, encryptErr
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
		return nil, insertErr
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return nil, nil
}

// Exports a local user as a private RSA key
// in a folder called export using the spec PEM format.
func Export(cmd Command, username, pass string) ([][]byte, error) {
	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return nil, existsErr
	}
	if !found {
		return nil, ErrorUserNotFound
	}

	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return nil, localUserErr
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return nil, ErrorWrongCredentials
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, decryptErr := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if decryptErr != nil {
		return nil, decryptErr
	}
	localUser.PrvKey = string(dec)

	// Creates export/ directory if it does not exist
	if _, err := os.Stat("export/"); os.IsNotExist(err) {
		os.Mkdir("export", 0755)
	}

	file := "export/" + username + ".priv"
	f, createErr := os.Create(file)
	if createErr != nil {
		return nil, createErr
	}
	defer f.Close()

	_, writeErr := f.Write([]byte(localUser.PrvKey))
	if writeErr != nil {
		return nil, writeErr
	}

	str := fmt.Sprintf(
		"file succesfully written to %s", f.Name(),
	)
	cmd.Output(str, RESULT)
	return nil, nil
}

// Changes the state of how TLS will be handled for a server.
func TLS(cmd Command, server *db.Server, on bool) ([][]byte, error) {
	if cmd.Data.IsConnected() {
		return nil, ErrorOfflineRequired
	}

	if on {
		server.TLS = true
		err := db.ChangeServerTLS(
			cmd.Static.DB,
			server.Address,
			server.Port,
			true,
		)

		if err != nil {
			return nil, err
		}

		return nil, nil
	} else {
		cmd.Data.Server.TLS = false
		err := db.ChangeServerTLS(
			cmd.Static.DB,
			server.Address,
			server.Port,
			false,
		)

		if err != nil {
			return nil, err
		}
		return nil, nil
	}
}

// Starts a connection with a server. If noverify is set,
// in case of TLS connections, certificate origins wont be checked.
// This command does not spawn a listening thread.
func Conn(cmd Command, server db.Server, noverify bool) ([][]byte, error) {
	if cmd.Data.IsConnected() {
		return nil, ErrorAlreadyConnected
	}

	useTLS := server.TLS
	skipVerify := false

	if noverify {
		if !useTLS {
			return nil, ErrorInvalidSkipVerify
		}

		skipVerify = true
		verbosePrint("certificate verification is going to be skipped!", cmd)
	}

	con, conErr := SocketConnect(
		server.Address,
		server.Port,
		useTLS,
		skipVerify,
	)
	if conErr != nil {
		return nil, conErr
	}

	cmd.Data.Conn = con
	err := WaitConnect(cmd, server)

	if err != nil {
		return nil, err
	}

	cmd.Output("listening for incoming packets...", INFO)
	return nil, nil
}

// Registers a user to a server and also adds it to the client database.
func Reg(ctx context.Context, cmd Command, username, pass string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	exists, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		string(username),
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return nil, existsErr
	}
	if exists {
		return nil, ErrorUserExists
	}

	// Generates the PEM arrays of both the private and public key of the pair
	verbosePrint("generating RSA key pair...", cmd)
	pair, rsaErr := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if rsaErr != nil {
		return nil, rsaErr
	}

	prvKeyPEM := spec.PrivkeytoPEM(pair)
	pubKeyPEM, pubKeyPEMErr := spec.PubkeytoPEM(&pair.PublicKey)
	if pubKeyPEMErr != nil {
		return nil, pubKeyPEMErr
	}

	// Hashes the provided password
	verbosePrint("hashing password...", cmd)
	hashPass, hashErr := bcrypt.GenerateFromPassword([]byte(pass), 12)
	if hashErr != nil {
		return nil, hashErr
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
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	// Awaits a response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	// Encrypts the private key
	verbosePrint("encrypting private key...", cmd)
	enc, err := db.EncryptData([]byte(pass), prvKeyPEM)
	if err != nil {
		return nil, err
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
		return nil, insertErr
	}

	cmd.Output(fmt.Sprintf(
		"local user %s successfully added to the database",
		username,
	), RESULT)
	return nil, nil
}

// Deregisters a user from the server and also removes it locally.
func Dereg(ctx context.Context, cmd Command, username, pass string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return nil, existsErr
	}
	if !found {
		return nil, ErrorUserNotFound
	}

	// Verifies password
	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return nil, localUserErr
	}
	// localUser.User = user

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return nil, ErrorWrongCredentials
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.DEREG, id, spec.EmptyInfo)
	if pctErr != nil {
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	dbErr := db.DeleteLocalUser(cmd.Static.DB, username, cmd.Data.Server.Address, cmd.Data.Server.Port)
	if dbErr != nil {
		return nil, dbErr
	}

	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Output(fmt.Sprintf("user %s deregistered correctly", username), RESULT)
	return nil, nil
}

// Logs a user to a server, also performs the verification.
func Login(ctx context.Context, cmd Command, username, pass string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if cmd.Data.IsLoggedIn() {
		return nil, ErrorAlreadyLoggedIn
	}

	found, existsErr := db.LocalUserExists(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if existsErr != nil {
		return nil, existsErr
	}
	if !found {
		return nil, ErrorUserNotFound
	}

	// Verifies password
	localUser, localUserErr := db.GetLocalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if localUserErr != nil {
		return nil, localUserErr
	}

	verbosePrint("checking password...", cmd)
	hash := []byte(localUser.Password)
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(pass))
	if cmpErr != nil {
		return nil, ErrorWrongCredentials
	}

	// Get the decrypted private key
	verbosePrint("decrypting private key...", cmd)
	dec, err := db.DecryptData([]byte(pass), []byte(localUser.PrvKey))
	if err != nil {
		return nil, err
	}
	localUser.PrvKey = string(dec)

	// Sends a LOGIN packet with the username as an argument
	verbosePrint("performing login...", cmd)
	id1 := cmd.Data.NextID()
	loginPct, loginPctErr := spec.NewPacket(
		spec.LOGIN, id1,
		spec.EmptyInfo, []byte(username),
	)
	if loginPctErr != nil {
		return nil, loginPctErr
	}

	if cmd.Static.Verbose {
		packetPrint(loginPct, cmd)
	}

	// Sends the packet
	_, loginWErr := cmd.Data.Conn.Write(loginPct)
	if loginWErr != nil {
		return nil, loginWErr
	}

	verbosePrint("awaiting response...", cmd)
	loginReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id1, spec.VERIF, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if loginReply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(loginReply.HD.Info)
	}

	// The reply is a VERIF
	// Decrypts the message
	pKey, pemErr := spec.PEMToPrivkey([]byte(localUser.PrvKey))
	if pemErr != nil {
		return nil, pemErr
	}

	decrypted, decryptErr := spec.DecryptText([]byte(loginReply.Args[0]), pKey)
	if decryptErr != nil {
		return nil, decryptErr
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
		return nil, verifPctErr
	}

	if cmd.Static.Verbose {
		packetPrint(verifPct, cmd)
	}

	// Sends the packet
	_, verifWErr := cmd.Data.Conn.Write(verifPct)
	if verifWErr != nil {
		return nil, verifWErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	verifReply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id2, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if verifReply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(verifReply.HD.Info)
	}
	verbosePrint("verification successful", cmd)
	// Assigns the logged in user to Data
	cmd.Data.User = &localUser

	cmd.Output(fmt.Sprintf("login successful. Welcome, %s", username), RESULT)

	perms, err := GetPermissions(ctx, cmd, localUser.User.Username)
	if err == nil {
		str := fmt.Sprintf(
			"logged in with permission level %d",
			perms,
		)
		cmd.Output(str, RESULT)
	}

	return nil, nil
}

// Logs out a user from a server.
func Logout(ctx context.Context, cmd Command) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}
	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.LOGOUT, id, spec.EmptyInfo)
	if pctErr != nil {
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, pctWErr := cmd.Data.Conn.Write(pct)
	if pctWErr != nil {
		return nil, pctWErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	// Empties the user value in Data
	cmd.Data.User = nil

	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Output("logged out", RESULT)
	return nil, nil
}

// Disconnects a client from a server.
func Discn(cmd Command) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	err := cmd.Data.Conn.Close()
	if err != nil {
		return nil, err
	}

	// Closes the shell client session
	cmd.Data.Conn = nil
	cmd.Data.User = nil
	cmd.Data.Waitlist.Cancel(cmd.Data.Logout)
	cmd.Data.Waitlist.Clear()
	cmd.Output("sucessfully disconnected from the server", RESULT)

	return nil, nil
}

// Sends a message to a user with the current time stamp and stores it in the database.
func Msg(ctx context.Context, cmd Command, username, message string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
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
		return nil, existsErr
	}
	if !found {
		return nil, ErrorUserNotFound
	}
	// Retrieves the public key in PEM format to encrypt the message
	externalUser, externalUserErr := db.GetExternalUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if externalUserErr != nil {
		return nil, externalUserErr
	}
	pubKey, pemErr := spec.PEMToPubkey([]byte(externalUser.PubKey))
	if pemErr != nil {
		return nil, pemErr
	}
	// Encrypts the text
	encrypted, encryptErr := spec.EncryptText([]byte(message), pubKey)
	if encryptErr != nil {
		return nil, encryptErr
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
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	// Sends the packet
	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	// Listens for response
	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("message sent correctly", RESULT)
	src, srcErr := db.GetUser(
		cmd.Static.DB,
		cmd.Data.User.User.Username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if srcErr != nil {
		return nil, srcErr
	}

	dst, dstErr := db.GetUser(
		cmd.Static.DB,
		username,
		cmd.Data.Server.Address,
		cmd.Data.Server.Port,
	)
	if dstErr != nil {
		return nil, dstErr
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
		return nil, storeErr
	}

	return nil, nil
}

// Asks the server to retrieve all messages while the user was offline.
// This function is not responsible for receiving the messages, only request them.
func Reciv(ctx context.Context, cmd Command) ([][]byte, error) {
	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.RECIV, id, spec.EmptyInfo)
	if pctErr != nil {
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("messages queried correctly", RESULT)
	return nil, nil
}

// Requests a list of users depending on the type specified, which may or not
// require an active connection.
// Returns a the received usernames in an array if the request was correct.
func Usrs(ctx context.Context, cmd Command, usrsType USRSType) ([][]byte, error) {
	if usrsType == LOCAL_ALL {
		users, err := printAllLocalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	}

	if usrsType == REQUESTED {
		users, err := printExternalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	}

	if usrsType == LOCAL_SERVER {
		users, err := printServerLocalUsers(cmd)
		if err != nil {
			return nil, err
		}
		return users, nil
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.USRS, id, byte(usrsType))
	if pctErr != nil {
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

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

	optionString := "all"
	if usrsType == ONLINE {
		optionString = "online"
	}

	cmd.Output(fmt.Sprintf("%s users:", optionString), USRS)
	cmd.Output(string(reply.Args[0]), USRS)
	split := bytes.Split(reply.Args[0], []byte("\n"))

	return split, nil
}

// Requests the information of an external user to add it to the client database.
// Returns the arguments of a REQ as by specification.
func Req(ctx context.Context, cmd Command, username string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotConnected
	}

	if username == cmd.Data.User.User.Username {
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

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

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
func Admin(ctx context.Context, cmd Command, op spec.Admin, args [][]byte) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	min, ok := adminArgs[op]

	if !ok {
		return nil, ErrorInvalidAdminOperation
	}

	if len(args) < int(min) {
		return nil, ErrorInsuficientArgs
	}

	id := cmd.Data.NextID()
	pct, pctErr := spec.NewPacket(spec.ADMIN, id, uint8(op), args...)
	if pctErr != nil {
		return nil, pctErr
	}

	if cmd.Static.Verbose {
		packetPrint(pct, cmd)
	}

	_, wErr := cmd.Data.Conn.Write(pct)
	if wErr != nil {
		return nil, wErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, err := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if err != nil {
		return nil, err
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output(fmt.Sprintf("admin operation %s sent successfully", spec.AdminString(op)), RESULT)
	return nil, nil
}

// Subscribes to a specific hook to the server.
func Sub(ctx context.Context, cmd Command, name string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	hook, ok := hooksList[name]
	if !ok {
		return nil, ErrorUnknownHookOption
	}

	str := fmt.Sprintf("subscribing to event %s...", name)
	verbosePrint(str, cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.SUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return nil, hookPctErr
	}

	if cmd.Static.Verbose {
		packetPrint(hookPct, cmd)
	}

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return nil, hookWErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, replyErr := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if replyErr != nil {
		return nil, replyErr
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("succesfully subscribed!", RESULT)
	return nil, nil
}

// Unsubscribes from a specific hook on the server.
func Unsub(ctx context.Context, cmd Command, name string) ([][]byte, error) {
	if !cmd.Data.IsConnected() {
		return nil, ErrorNotConnected
	}

	if !cmd.Data.IsLoggedIn() {
		return nil, ErrorNotLoggedIn
	}

	hook, ok := hooksList[name]
	if !ok {
		return nil, ErrorUnknownHookOption
	}

	str := fmt.Sprintf("unsubscribing to event %s...", name)
	verbosePrint(str, cmd)
	id := cmd.Data.NextID()
	hookPct, hookPctErr := spec.NewPacket(
		spec.UNSUB, id,
		byte(hook),
	)
	if hookPctErr != nil {
		return nil, hookPctErr
	}

	if cmd.Static.Verbose {
		packetPrint(hookPct, cmd)
	}

	_, hookWErr := cmd.Data.Conn.Write(hookPct)
	if hookWErr != nil {
		return nil, hookWErr
	}

	verbosePrint("awaiting response...", cmd)
	reply, replyErr := cmd.Data.Waitlist.Get(
		ctx, Find(id, spec.OK, spec.ERR),
	)
	if replyErr != nil {
		return nil, replyErr
	}

	if reply.HD.Op == spec.ERR {
		return nil, spec.ErrorCodeToError(reply.HD.Info)
	}

	cmd.Output("succesfully unsubscribed!", RESULT)
	return nil, nil
}
