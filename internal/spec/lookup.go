package spec

/* PREDEFINED VALUES */

const (
	ProtocolVersion uint8  = 1         // Current version of the protocol
	MaxClients      int    = 20        // Max amount of clients the server allows at the same time
	NullOp          Action = 0         // Invalid operation code
	NullID          ID     = 0         // Only valid for specific documented cases
	MaxID           ID     = 1<<10 - 1 // Maximum value according to the bit field
	EmptyInfo       byte   = 0xFF      // No information provided
	HeaderSize      int    = 8         // Max size of the header in bytes
	MaxArgs         int    = 1<<4 - 1  // Max amount of arguments
	MaxPayload      int    = 1<<14 - 1 // Max amount of total arguments size
	MaxArgSize      int    = 1<<11 - 1 // Max amount of single argument size
	RSABitSize      int    = 4096      // Size of the RSA keypair used by the spec crypto functions
	UsernameSize    int    = 32        // Max size of a username in bytes
	LoginTimeout    int    = 2         // Timeout for a handshake process in minutes
	ReadTimeout     int    = 10        // Timeout for a TCP read block in minutes
	TokenExpiration int    = 30        // Deadline for a reusable token expiration in minutes
)

/* ACTION CODES */

// Specifies an operation to be performed.
type Action uint8

// The integer follows the actual binary value of the operation.
const (
	OK Action = iota + 1
	ERR
	REG
	VERIF
	REQ
	USRS
	RECIV
	LOGIN
	MSG
	LOGOUT
	DEREG
	SHTDWN
	ADMIN
	KEEP
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
	regLookup    = lookup{REG, 0x03, "REG", 2, -1}
	verifLookup  = lookup{VERIF, 0x04, "VERIF", 2, 1}
	reqLookup    = lookup{REQ, 0x05, "REQ", 1, 2}
	usrsLookup   = lookup{USRS, 0x06, "USRS", 0, 1}
	recivLookup  = lookup{RECIV, 0x07, "RECIV", 0, 3}
	loginLookup  = lookup{LOGIN, 0x08, "LOGIN", 1, -1}
	msgLookup    = lookup{MSG, 0x09, "MSG", 3, -1}
	logoutLookup = lookup{LOGOUT, 0x0A, "LOGOUT", 0, -1}
	deregLookup  = lookup{DEREG, 0x0B, "DEREG", 0, -1}
	shtdwnLookup = lookup{SHTDWN, 0x0C, "SHTDWN", 0, -1}
	adminLookup  = lookup{ADMIN, 0x0D, "ADMIN", 0, -1}
	keepLookup   = lookup{KEEP, 0x0E, "KEEP", 0, -1}
)

var lookupByOperation map[Action]lookup = map[Action]lookup{
	OK:     okLookup,
	ERR:    errLookup,
	REG:    regLookup,
	VERIF:  verifLookup,
	REQ:    reqLookup,
	USRS:   usrsLookup,
	RECIV:  recivLookup,
	LOGIN:  loginLookup,
	MSG:    msgLookup,
	LOGOUT: logoutLookup,
	DEREG:  deregLookup,
	SHTDWN: shtdwnLookup,
	ADMIN:  adminLookup,
	KEEP:   keepLookup,
}

var lookupByString map[string]lookup = map[string]lookup{
	"OK":     okLookup,
	"ERR":    errLookup,
	"REG":    regLookup,
	"VERIF":  verifLookup,
	"REQ":    reqLookup,
	"USRS":   usrsLookup,
	"RECIV":  recivLookup,
	"LOGIN":  loginLookup,
	"MSG":    msgLookup,
	"LOGOUT": logoutLookup,
	"DEREG":  deregLookup,
	"SHTDWN": shtdwnLookup,
	"ADMIN":  adminLookup,
	"KEEP":   keepLookup,
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
	Code uint8
	Text string
}

// Returns the text asocciated to the error.
func (err SpecError) Error() string {
	return err.Text
}

var (
	ErrorUndefined    error = SpecError{0x00, "undefined problem occured"}               // undefined problem occured
	ErrorInvalid      error = SpecError{0x01, "invalid operation performed"}             // invalid operation performed
	ErrorNotFound     error = SpecError{0x02, "content can not be found"}                // content can not be found
	ErrorVersion      error = SpecError{0x03, "server and client versions do not match"} // server and client versions do not match
	ErrorHandshake    error = SpecError{0x04, "handshake process failed"}                // handshake process failed
	ErrorArguments    error = SpecError{0x05, "invalid arguments given"}                 // invalid arguments given
	ErrorMaxSize      error = SpecError{0x06, "data size is too big"}                    // data size is too big
	ErrorHeader       error = SpecError{0x07, "invalid header provided"}                 // invalid header provided
	ErrorNoSession    error = SpecError{0x08, "user is not connected"}                   // user is not connected
	ErrorLogin        error = SpecError{0x09, "user can not be logged in"}               // user can not be logged in
	ErrorConnection   error = SpecError{0x0A, "connection problem occured"}              // connection problem occured
	ErrorEmpty        error = SpecError{0x0B, "queried data is empty"}                   // queried data is empty
	ErrorPacket       error = SpecError{0x0C, "packet could not be delivered"}           // packet could not be delivered
	ErrorPrivileges   error = SpecError{0x0D, "missing privileges to run"}               // missing privileges to run
	ErrorServer       error = SpecError{0x0E, "server operation failed"}                 // server operation failed
	ErrorIdle         error = SpecError{0x0F, "user has been idle for too long"}         // user has been idle for too long
	ErrorExists       error = SpecError{0x10, "content already exists"}                  // content already exists
	ErrorUnescure     error = SpecError{0x10, "connection is not secure"}                // connection is not secure
	ErrorDeregistered error = SpecError{0x11, "user no longer exists"}                   // user no longer exists
	ErrorDupSession   error = SpecError{0x12, "session exists in another endpoint"}      // session exists in another endpoint
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

const (
	AdminShutdown   = 0x00 // Send a shutdown signal to the server
	AdminDeregister = 0x01 // Force the deregistration of a user
	AdminBroadcast  = 0x02 // Broadcast a message to all online users
	AdminPromote    = 0x03 // Increase the permission level of a user
	AdminDisconnect = 0x04 // Disconnect an online user
)

var codeToAdmin map[byte]string = map[byte]string{
	0x00: "ADMIN_SHTDWN",
	0x01: "ADMIN_DEREG",
	0x02: "ADMIN_BRDCAST",
	0x03: "ADMIN_PROMOTE",
	0x04: "ADMIN_KICK",
}

// Returns the admin string asocciated to a hex byte.
// Result is an empty string if not found.
func AdminString(a uint8) string {
	v, ok := codeToAdmin[a]
	if !ok {
		return ""
	}
	return v
}
