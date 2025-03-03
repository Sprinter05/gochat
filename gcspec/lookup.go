package gcspec

/* PREDEFINED VALUES */

const NullOp Action = 0
const NullID ID = 0
const MaxID ID = 1<<10 - 1
const EmptyInfo byte = 0xFF

const HeaderSize int = 8
const MaxArgs int = 1<<4 - 1
const MaxPayload int = 1<<14 - 1
const MaxArgSize int = 1<<11 - 1

const ProtocolVersion uint8 = 1
const RSABitSize int = 4096
const UsernameSize int = 32

const LoginTimeout int = 2 // Minutes
const ReadTimeout int = 10 // Minutes
const MaxClients int = 20

/* ACTION CODES */

// Specifies an action code
type Action uint8

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
// sargs indicates the arguments to send to server
// cargs indicates the arguments to send to client
type lookup struct {
	op    Action
	hex   uint8
	str   string
	sargs int8
	cargs int8
}

// -1 indicates the command cannot be used for client and/or server
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

// Args represents the amount of arguments the server needs
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

// Returns the ID associated to a byte code
func CodeToID(b byte) Action {
	v, ok := lookupByOperation[Action(b)]
	if !ok {
		return NullOp
	}
	return v.op
}

// Returns the byte code asocciated to an ID
func IDToCode(a Action) byte {
	v, ok := lookupByOperation[a]
	if !ok {
		return 0x0
	}
	return v.hex
}

// Returns the ID associated to a string
func StringToCode(s string) Action {
	v, ok := lookupByString[s]
	if !ok {
		return 0x0
	}
	return v.op
}

// Returns the ID associated to a string
func CodeToString(a Action) string {
	v, ok := lookupByOperation[a]
	if !ok {
		return ""
	}
	return v.str
}

// Minimum amount of arguments to send to server
func ServerArgs(a Action) int {
	v, ok := lookupByOperation[a]
	if !ok {
		return -1
	}
	return int(v.sargs)
}

// Minimum amount of arguments to send to client
func ClientArgs(a Action) int {
	v, ok := lookupByOperation[a]
	if !ok {
		return -1
	}
	return int(v.cargs)
}

/* ERROR CODES */

// Specific GCError struct that implements the error interface
type GCError struct {
	Code uint8
	Text string
}

func (err GCError) Error() string {
	return err.Text
}

var (
	ErrorUndefined  error = GCError{0x00, "undefined problem occured"}
	ErrorInvalid    error = GCError{0x01, "invalid operation performed"}
	ErrorNotFound   error = GCError{0x02, "content can not be found"}
	ErrorVersion    error = GCError{0x03, "server and client versions do not match"}
	ErrorHandshake  error = GCError{0x04, "handshake process failed"}
	ErrorArguments  error = GCError{0x05, "invalid arguments given"}
	ErrorMaxSize    error = GCError{0x06, "data size is too big"}
	ErrorHeader     error = GCError{0x07, "invalid header provided"}
	ErrorNoSession  error = GCError{0x08, "user is not connected"}
	ErrorLogin      error = GCError{0x09, "user can not be logged in"}
	ErrorConnection error = GCError{0x0A, "connection problem occured"}
	ErrorEmpty      error = GCError{0x0B, "queried data is empty"}
	ErrorPacket     error = GCError{0x0C, "packet could not be delivered"}
	ErrorPrivileges error = GCError{0x0D, "missing privileges to run"}
	ErrorServer     error = GCError{0x0E, "server operation failed"}
	ErrorIdle       error = GCError{0x0F, "user has been idle for too long"}
	ErrorExists     error = GCError{0x10, "content already exists"}
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
}

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	switch v := err.(type) {
	case GCError:
		return v.Code
	default:
		return EmptyInfo
	}
}

// Returns the error or the nil if not found
func ErrorCodeToError(b byte) error {
	v, ok := codeToError[b]
	if !ok {
		return nil
	}
	return v
}

/* ADMIN OPERATIONS */

const (
	AdminShutdown   = 0x00
	AdminDeregister = 0x01
	AdminBroadcast  = 0x02
	AdminPromote    = 0x03
	AdminDisconnect = 0x04
)

var codeToAdmin map[byte]string = map[byte]string{
	0x00: "ADMIN_SHTDWN",
	0x01: "ADMIN_DEREG",
	0x02: "ADMIN_BRDCAST",
	0x03: "ADMIN_PROMOTE",
	0x04: "ADMIN_KICK",
}

// Returns the error code or the empty information field if not found
func AdminString(a uint8) string {
	v, ok := codeToAdmin[a]
	if !ok {
		return ""
	}
	return v
}
