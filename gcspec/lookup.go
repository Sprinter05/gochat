package gcspec

/* PREDEFINED VALUES */

const NullOp Action = 0
const NullID ID = 0
const EmptyInfo byte = 0xFF

const ProtocolVersion uint8 = 1
const HeaderSize int = 6
const MaxArgs int = 1<<2 - 1
const MaxPayload int = 1<<10 - 1
const RSABitSize int = 4096
const UsernameSize int = 32

const CypherCharset string = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz#$%&*+-?!"
const CypherLength int = 128

const LoginTimeout int = 2
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
	CONN
	MSG
	DISCN
	DEREG
	SHTDWN
	ADMIN
)

//? Reduce the amount of tables

var codeToid map[byte]Action = map[byte]Action{
	0x01: OK,
	0x02: ERR,
	0x03: REG,
	0x04: VERIF,
	0x05: REQ,
	0x06: USRS,
	0x07: RECIV,
	0x08: CONN,
	0x09: MSG,
	0x0A: DISCN,
	0x0B: DEREG,
	0x0C: SHTDWN,
	0x0D: ADMIN,
}

var idToCode map[Action]byte = map[Action]byte{
	OK:     0x01,
	ERR:    0x02,
	REG:    0x03,
	VERIF:  0x04,
	REQ:    0x05,
	USRS:   0x06,
	RECIV:  0x07,
	CONN:   0x08,
	MSG:    0x09,
	DISCN:  0x0A,
	DEREG:  0x0B,
	SHTDWN: 0x0C,
	ADMIN:  0x0D,
}

var stringToCode map[string]Action = map[string]Action{
	"OK":     OK,
	"ERR":    ERR,
	"REG":    REG,
	"VERIF":  VERIF,
	"REQ":    REQ,
	"USRS":   USRS,
	"RECIV":  RECIV,
	"CONN":   CONN,
	"MSG":    MSG,
	"DISCN":  DISCN,
	"DEREG":  DEREG,
	"SHTDWN": SHTDWN,
	"ADMIN":  ADMIN,
}

var codeToString map[Action]string = map[Action]string{
	OK:     "OK",
	ERR:    "ERR",
	REG:    "REG",
	VERIF:  "VERIF",
	REQ:    "REQ",
	USRS:   "USRS",
	RECIV:  "RECIV",
	CONN:   "CONN",
	MSG:    "MSG",
	DISCN:  "DISCN",
	DEREG:  "DEREG",
	SHTDWN: "SHTDWN",
	ADMIN:  "ADMIN",
}

// Returns the ID associated to a byte code
func CodeToID(b byte) Action {
	v, ok := codeToid[b]
	if !ok {
		return NullOp
	}
	return v
}

// Returns the byte code asocciated to an ID
func IDToCode(a Action) byte {
	v, ok := idToCode[a]
	if !ok {
		return 0x0
	}
	return v
}

// Returns the ID associated to a string
func StringToCode(s string) Action {
	v, ok := stringToCode[s]
	if !ok {
		return 0x0
	}
	return v
}

// Returns the ID associated to a string
func CodeToString(i Action) string {
	v, ok := codeToString[i]
	if !ok {
		return ""
	}
	return v
}

/* ARGUMENTS PER OPERATION */

var idToArgs map[Action]uint8 = map[Action]uint8{
	OK:     0,
	ERR:    0,
	REG:    2,
	VERIF:  2,
	REQ:    1,
	USRS:   0,
	RECIV:  3,
	CONN:   1,
	MSG:    3,
	DISCN:  0,
	DEREG:  0,
	SHTDWN: 0,
	ADMIN:  0, // Special case, can have more arguments
}

func IDToArgs(a Action) int {
	v, ok := idToArgs[a]
	if !ok {
		return -1
	}
	return int(v)
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
	// Determines a generic undefined error
	ErrorUndefined error = GCError{0x0, "undefined problem occured"}

	// Invalid operation performed
	ErrorInvalid error = GCError{0x1, "invalid operation performed"}

	// Content could not be found
	ErrorNotFound error = GCError{0x2, "content can not be found"}

	// Versions do not match
	ErrorVersion error = GCError{0x3, "server and client versions do not match"}

	// Verification handshake failed
	ErrorHandshake error = GCError{0x4, "handshake process failed"}

	// Invalid arguments given
	ErrorArguments error = GCError{0x5, "invalid arguments given"}

	// Payload size too big
	ErrorMaxSize error = GCError{0x6, "size is too big"}

	// Header processing failed
	ErrorHeader error = GCError{0x7, "invalid header provided"}

	// User is not logged in
	ErrorNoSession error = GCError{0x8, "user is not connected"}

	// User cannot be logged in
	ErrorLogin error = GCError{0x9, "user can not be logged in"}

	// Connection problems occured
	ErrorConnection error = GCError{0xA, "connection problem occured"}

	// Empty result returned
	ErrorEmpty error = GCError{0xB, "queried data is empty"}

	// Problem with packet creation or delivery
	ErrorPacket error = GCError{0xC, "packet could not be delivered"}

	// Not enough privileges to runa ction
	ErrorPrivileges error = GCError{0x0D, "missing privileges to run"}

	// Failed to perform a server-side operation
	ErrorServer error = GCError{0x0E, "server operation failed"}
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
	// Schedules a shutdown the server
	AdminShutdown uint8 = 0x00

	// Deregisters a user manually
	AdminDeregister uint8 = 0x01

	// Broadcasts a message to all online users
	AdminBroadcast uint8 = 0x02

	// Increases the permission levels of another user
	AdminPromote uint8 = 0x03
)
