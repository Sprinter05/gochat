package spec

/* PREDEFINED VALUES */

const (
	ProtocolVersion  uint8  = 1                  // Current version of the protocol
	NullOp           Action = 0                  // Invalid operation code
	NullID           ID     = 0                  // Only valid for specific documented cases
	MaxID            ID     = 1<<10 - 1          // Maximum value according to the bit field
	EmptyInfo        byte   = 0xFF               // No information provided
	HeaderSize       int    = 8                  // Max size of the header in bytes
	MaxArgs          int    = (1 << 4) - 1       // Max amount of arguments
	MaxPayload       int    = (1 << 14) - 1      // Max amount of total arguments size
	MaxArgSize       int    = (1 << 11) - 1      // Max amount of single argument size
	RSABitSize       int    = 4096               // Size of the RSA keypair used by the spec crypto functions
	UsernameSize     int    = 32                 // Max size of a username in bytes
	LoginTimeout     int    = 2                  // Timeout for a handshake process in minutes
	ReadTimeout      int    = 10                 // Timeout for a TCP read block in minutes
	HandshakeTimeout int    = 20                 // Timeout for a connection handshake block in seconds
	TokenExpiration  int    = 30                 // Deadline for a reusable token expiration in minutes
	UsernameRegex    string = "^[0-9a-z]{0,32}$" // To check if ausername is valid
)

/* ACTION CODES */

// Specifies an operation to be performed.
type Action uint8

// The integer follows the actual binary value of the operation.
const (
	OK Action = iota + 1
	ERR
	KEEP
	REG
	DEREG
	LOGIN
	LOGOUT
	VERIF
	REQ
	USRS
	MSG
	RECIV
	SHTDWN
	ADMIN
	SUB
	UNSUB
	HOOK
	HELLO
)

// Identifies an operation to be performed
// with detailed information.
//
// -1 as minimum amount of argument indicates
// that the command cannot be used in that direction.
type lookup struct {
	op    Action // Operation code
	hex   uint8  // Operation code as binary hex
	str   string // Operation code as string
	sargs int8   // Minimum arguments to send to server
	cargs int8   // Minimum arguments to send to client
}

var (
	okLookup     = lookup{OK, 0x01, "OK", -1, 0}
	errLookup    = lookup{ERR, 0x02, "ERR", -1, 0}
	keepLookup   = lookup{KEEP, 0x03, "KEEP", 0, -1}
	regLookup    = lookup{REG, 0x04, "REG", 2, -1}
	deregLookup  = lookup{DEREG, 0x05, "DEREG", 0, -1}
	loginLookup  = lookup{LOGIN, 0x06, "LOGIN", 1, -1}
	logoutLookup = lookup{LOGOUT, 0x07, "LOGOUT", 0, -1}
	verifLookup  = lookup{VERIF, 0x08, "VERIF", 2, 1}
	reqLookup    = lookup{REQ, 0x09, "REQ", 1, 3}
	usrsLookup   = lookup{USRS, 0x0A, "USRS", 0, 1}
	msgLookup    = lookup{MSG, 0x0B, "MSG", 3, -1}
	recivLookup  = lookup{RECIV, 0x0C, "RECIV", 0, 3}
	shtdwnLookup = lookup{SHTDWN, 0x0D, "SHTDWN", -1, 0}
	adminLookup  = lookup{ADMIN, 0x0E, "ADMIN", 0, -1}
	subLookup    = lookup{SUB, 0x0F, "SUB", 0, -1}
	unsubLookup  = lookup{UNSUB, 0x10, "UNSUB", 0, -1}
	hookLookup   = lookup{HOOK, 0x11, "HOOK", -1, 0}
	helloLookup  = lookup{HELLO, 0x12, "HELLO", -1, 1}
)

var lookupByOperation map[Action]lookup = map[Action]lookup{
	OK:     okLookup,
	ERR:    errLookup,
	KEEP:   keepLookup,
	REG:    regLookup,
	DEREG:  deregLookup,
	LOGIN:  loginLookup,
	LOGOUT: logoutLookup,
	VERIF:  verifLookup,
	REQ:    reqLookup,
	USRS:   usrsLookup,
	MSG:    msgLookup,
	RECIV:  recivLookup,
	SHTDWN: shtdwnLookup,
	ADMIN:  adminLookup,
	SUB:    subLookup,
	UNSUB:  unsubLookup,
	HOOK:   hookLookup,
	HELLO:  helloLookup,
}

var lookupByString map[string]lookup = map[string]lookup{
	"OK":     okLookup,
	"ERR":    errLookup,
	"KEEP":   keepLookup,
	"REG":    regLookup,
	"DEREG":  deregLookup,
	"LOGIN":  loginLookup,
	"LOGOUT": logoutLookup,
	"VERIF":  verifLookup,
	"REQ":    reqLookup,
	"USRS":   usrsLookup,
	"MSG":    msgLookup,
	"RECIV":  recivLookup,
	"SHTDWN": shtdwnLookup,
	"ADMIN":  adminLookup,
	"SUB":    subLookup,
	"UNSUB":  unsubLookup,
	"HOOK":   hookLookup,
	"HELLO":  helloLookup,
}

// Returns the operation code associated to a hex byte.
// Result is NullOp if not found.
func CodeToID(b byte) Action {
	v, ok := lookupByOperation[Action(b)]
	if !ok {
		return NullOp
	}
	return v.op
}

// Returns the hex byte asocciated to an operation code.
// Result is 0x0 if not found.
func IDToCode(a Action) byte {
	v, ok := lookupByOperation[a]
	if !ok {
		return 0x0
	}
	return v.hex
}

// Returns the action code associated to a string.
// Result is NullOp if not found.
func StringToCode(s string) Action {
	v, ok := lookupByString[s]
	if !ok {
		return NullOp
	}
	return v.op
}

// Returns the string associated to an operation code.
// Result is an empty string if not found.
func CodeToString(a Action) string {
	v, ok := lookupByOperation[a]
	if !ok {
		return ""
	}
	return v.str
}

// Returns the minimum amount of arguments needed
// to send to the server.
// Result is -1 if it cannot be sent to the server.
func ServerArgs(a Action) int {
	v, ok := lookupByOperation[a]
	if !ok {
		return -1
	}
	return int(v.sargs)
}

// Returns the minimum amount of arguments needed
// to send to the client.
// Result is -1 if it cannot be sent to the client.
func ClientArgs(a Action) int {
	v, ok := lookupByOperation[a]
	if !ok {
		return -1
	}
	return int(v.cargs)
}

/* ERROR CODES */

// Error that implements the error interface from
// the [errors] package with specific information
// that follows the protocol specification.
type SpecError struct {
	Code        uint8
	Text        string
	Description string
}

// Returns the text asocciated to the error.
func (err SpecError) Error() string {
	return err.Description
}

var (
	ErrorUndefined    error = SpecError{0x00, "ERR_UNDEFINED", "undefined problem occured"}             // undefined problem occured
	ErrorInvalid      error = SpecError{0x01, "ERR_INVALID", "invalid operation performed"}             // invalid operation performed
	ErrorNotFound     error = SpecError{0x02, "ERR_NOTFOUND", "content can not be found"}               // content can not be found
	ErrorVersion      error = SpecError{0x03, "ERR_VERSION", "server and client versions do not match"} // server and client versions do not match
	ErrorHandshake    error = SpecError{0x04, "ERR_HANDSHAKE", "handshake process failed"}              // handshake process failed
	ErrorArguments    error = SpecError{0x05, "ERR_ARGS", "invalid arguments given"}                    // invalid arguments given
	ErrorMaxSize      error = SpecError{0x06, "ERR_MAXSIZE", "data size is too big"}                    // data size is too big
	ErrorHeader       error = SpecError{0x07, "ERR_HEADER", "invalid header provided"}                  // invalid header provided
	ErrorNoSession    error = SpecError{0x08, "ERR_NOSESS", "user is not connected"}                    // user is not connected
	ErrorLogin        error = SpecError{0x09, "ERR_LOGIN", "user can not be logged in"}                 // user can not be logged in
	ErrorConnection   error = SpecError{0x0A, "ERR_CONN", "connection problem occured"}                 // connection problem occured
	ErrorEmpty        error = SpecError{0x0B, "ERR_EMPTY", "queried data is empty"}                     // queried data is empty
	ErrorPacket       error = SpecError{0x0C, "ERR_PACKET", "packet could not be delivered"}            // packet could not be delivered
	ErrorPrivileges   error = SpecError{0x0D, "ERR_PERMS", "missing privileges to run"}                 // missing privileges to run
	ErrorServer       error = SpecError{0x0E, "ERR_SERVER", "server operation failed"}                  // server operation failed
	ErrorIdle         error = SpecError{0x0F, "ERR_IDLE", "user has been idle for too long"}            // user has been idle for too long
	ErrorExists       error = SpecError{0x10, "ERR_EXISTS", "content already exists"}                   // content already exists
	ErrorDeregistered error = SpecError{0x11, "ERR_DEREG", "user has been deleted"}                     // user has been deleted
	ErrorDupSession   error = SpecError{0x12, "ERR_DUPSESS", "session exists in another endpoint"}      // session exists in another endpoint
	ErrorUnsecure     error = SpecError{0x13, "ERR_NOSECURE", "secured connection required"}            // secure connection required
	ErrorCorrupted    error = SpecError{0x14, "ERR_CORRUPTED", "queried data is currupted"}             // queried data is corrupted
	ErrorOption       error = SpecError{0x15, "ERR_OPTION", "invalid option provided"}                  // invalid option provided
	ErrorDisconnected error = SpecError{0x16, "ERR_DISCN", "connection was manually closed"}            // connection manually closed
)

var codeToError map[byte]error = map[byte]error{
	0x00: ErrorUndefined,
	0x01: ErrorInvalid,
	0x02: ErrorNotFound,
	0x03: ErrorVersion,
	0x04: ErrorHandshake,
	0x05: ErrorArguments,
	0x06: ErrorMaxSize,
	0x07: ErrorHeader,
	0x08: ErrorNoSession,
	0x09: ErrorLogin,
	0x0A: ErrorConnection,
	0x0B: ErrorEmpty,
	0x0C: ErrorPacket,
	0x0D: ErrorPrivileges,
	0x0E: ErrorServer,
	0x0F: ErrorIdle,
	0x10: ErrorExists,
	0x11: ErrorDeregistered,
	0x12: ErrorDupSession,
	0x13: ErrorUnsecure,
	0x14: ErrorCorrupted,
	0x15: ErrorOption,
	0x16: ErrorDisconnected,
}

// Returns the error asocciated to a hex byte.
// Result is EmptyInfo if not found.
func ErrorCode(err error) byte {
	switch v := err.(type) {
	case SpecError:
		return v.Code
	default:
		return EmptyInfo
	}
}

// Returns the formal error string as defined by the
// implementation. If the provided error is not a spec
// error an empty string is returned.
func ErrorString(err error) string {
	switch v := err.(type) {
	case SpecError:
		return v.Text
	default:
		return ""
	}
}

// Returns the hex byte asocciated to an error.
// Result is nil if not found.
func ErrorCodeToError(b byte) error {
	v, ok := codeToError[b]
	if !ok {
		return nil
	}
	return v
}

/* ADMIN OPERATIONS */

// Specifies an admin operation to be performed
type Admin uint8

const (
	AdminShutdown    Admin = 0x00 // Send a shutdown signal to the server
	AdminDeregister  Admin = 0x01 // Force the deregistration of a user
	AdminBroadcast   Admin = 0x02 // Broadcast a message to all online users
	AdminChangePerms Admin = 0x03 // Increase the permission level of a user
	AdminDisconnect  Admin = 0x04 // Disconnect an online user
	AdminMotd        Admin = 0x05 // Changes the MOTD of the server
)

var codeToAdmin map[Admin]string = map[Admin]string{
	AdminShutdown:    "ADMIN_SHTDWN",
	AdminDeregister:  "ADMIN_DEREG",
	AdminBroadcast:   "ADMIN_BRDCAST",
	AdminChangePerms: "ADMIN_CHGPERMS",
	AdminDisconnect:  "ADMIN_KICK",
	AdminMotd:        "ADMIN_MOTD",
}

var adminToArgs map[Admin]int = map[Admin]int{
	AdminShutdown:    1,
	AdminDeregister:  1,
	AdminBroadcast:   1,
	AdminChangePerms: 2,
	AdminDisconnect:  1,
	AdminMotd:        1,
}

// Returns the admin string asocciated to a hex byte.
// Result is an empty string if not found.
func AdminString(a Admin) string {
	v, ok := codeToAdmin[a]
	if !ok {
		return ""
	}
	return v
}

// Returns the amount of arguments the hook should have
// Result is -1 if not found
func AdminArgs(a Admin) int {
	v, ok := adminToArgs[a]
	if !ok {
		return -1
	}

	return v
}

/* HOOKS */

// Specifies a hook that triggers on a specific event
type Hook uint8

const (
	HookAllHooks         Hook = 0x00 // Subscribe or unsubscribe to all existing hooks
	HookNewLogin         Hook = 0x01 // Triggers when a user comes online
	HookNewLogout        Hook = 0x02 // Triggers when a user goes offline
	HookDuplicateSession Hook = 0x03 // Triggers when a session for the user is opened from another endpoint
	HookPermsChange      Hook = 0x04 // Triggers when a user's permission level changes
)

// Array with all possible existing hooks for easier traversal
var Hooks []Hook = []Hook{
	HookNewLogin,
	HookNewLogout,
	HookDuplicateSession,
	HookPermsChange,
}

var codeToHook map[Hook]string = map[Hook]string{
	HookAllHooks:         "HOOK_ALL",
	HookNewLogin:         "HOOK_NEWLOGIN",
	HookNewLogout:        "HOOK_NEWLOGOUT",
	HookDuplicateSession: "HOOK_DUPSESS",
	HookPermsChange:      "HOOK_PERMSCHG",
}

var hookToArgs map[Hook]int = map[Hook]int{
	HookNewLogin:         2,
	HookNewLogout:        1,
	HookDuplicateSession: 1,
	HookPermsChange:      2,
}

// Returns the hook string asocciated to a hex byte.
// Result is an empty string if not found.
func HookString(h Hook) string {
	v, ok := codeToHook[h]
	if !ok {
		return ""
	}
	return v
}

// Returns the amount of arguments the hook should have
// Result is -1 if not found
func HookArgs(h Hook) int {
	v, ok := hookToArgs[h]
	if !ok {
		return -1
	}

	return v
}

/* USER LISTING */

// Specifies the user option for the command
type Userlist uint8

const (
	UsersAll         Userlist = 0x0
	UsersOnline      Userlist = 0x1
	UsersAllPerms    Userlist = 0x2
	UsersOnlinePerms Userlist = 0x3
)

var userToOption map[Userlist]string = map[Userlist]string{
	UsersAll:         "USRS_ALL",
	UsersOnline:      "USRS_ONLINE",
	UsersAllPerms:    "USRS_ALLPERMS",
	UsersOnlinePerms: "USRS_ONLINEPERMS",
}

func UserlistString(u Userlist) string {
	v, ok := userToOption[u]
	if !ok {
		return ""
	}
	return v
}
