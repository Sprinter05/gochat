package gcspec

import (
	"errors"
)

/* PREDEFINED VALUES */

const NullID Action = 0
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

// Returns the ID associated to a byte code
func CodeToID(b byte) Action {
	v, ok := codeToid[b]
	if !ok {
		return NullID
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

/* ERROR CODES */

// Determines a generic undefined error
var ErrorUndefined error = errors.New("undefined problem")

// Invalid operation performed
var ErrorInvalid error = errors.New("invalid operation performed")

// Content could not be found
var ErrorNotFound error = errors.New("content not found")

// Versions do not match
var ErrorVersion error = errors.New("versions do not match")

// Verification handshake failed
var ErrorHandshake error = errors.New("handshake failed")

// Invalid arguments given
var ErrorArguments error = errors.New("invalid arguments")

// Payload size too big
var ErrorMaxSize error = errors.New("payload size too big")

// Header processing failed
var ErrorHeader error = errors.New("invalid header provided")

// User is not logged in
var ErrorNoSession error = errors.New("user is not connected")

// User cannot be logged in
var ErrorLogin error = errors.New("user can not be logged in")

var errorCodes map[error]byte = map[error]byte{
	ErrorUndefined: 0x00,
	ErrorInvalid:   0x01,
	ErrorNotFound:  0x02,
	ErrorVersion:   0x03,
	ErrorHandshake: 0x04,
	ErrorArguments: 0x05,
	ErrorMaxSize:   0x06,
	ErrorHeader:    0x07,
	ErrorNoSession: 0x08,
	ErrorLogin:     0x09,
}

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
