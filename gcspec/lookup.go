package gcspec

import (
	"errors"
)

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
)

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
}

func IDToArgs(a Action) int {
	v, ok := idToArgs[a]
	if !ok {
		return -1
	}
	return int(v)
}

/* ERROR CODES */

// TODO: Use an interface

// Determines a generic undefined error
var ErrorUndefined error = errors.New("undefined problem occured")

// Invalid operation performed
var ErrorInvalid error = errors.New("invalid operation performed")

// Content could not be found
var ErrorNotFound error = errors.New("content can not be found")

// Versions do not match
var ErrorVersion error = errors.New("server and client versions do not match")

// Verification handshake failed
var ErrorHandshake error = errors.New("handshake process failed")

// Invalid arguments given
var ErrorArguments error = errors.New("invalid arguments given")

// Payload size too big
var ErrorMaxSize error = errors.New("size is too big")

// Header processing failed
var ErrorHeader error = errors.New("invalid header provided")

// User is not logged in
var ErrorNoSession error = errors.New("user is not connected")

// User cannot be logged in
var ErrorLogin error = errors.New("user can not be logged in")

// Connection problems occured
var ErrorConnection error = errors.New("connection problem occured")

// Empty result returned
var ErrorEmpty error = errors.New("queried data is empty")

var errorCodes map[error]byte = map[error]byte{
	ErrorUndefined:  0x00,
	ErrorInvalid:    0x01,
	ErrorNotFound:   0x02,
	ErrorVersion:    0x03,
	ErrorHandshake:  0x04,
	ErrorArguments:  0x05,
	ErrorMaxSize:    0x06,
	ErrorHeader:     0x07,
	ErrorNoSession:  0x08,
	ErrorLogin:      0x09,
	ErrorConnection: 0x0A,
	ErrorEmpty:      0x0B,
}

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
