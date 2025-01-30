package gcspec

import (
	"errors"
)

// PREDEFINED VALUES

const NullID ID = 0
const EmptyInfo byte = 0xFF

const ProtocolVersion uint = 1
const HeaderSize int = 4
const MaxArgs int = 1<<2 - 1
const MaxPayload int = 1<<10 - 1

// ACTION CODES

type ID uint8

const (
	OK ID = iota + 1
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

var codeToid map[byte]ID = map[byte]ID{
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

var idToCode map[ID]byte = map[ID]byte{
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
func CodeToID(b byte) ID {
	v, ok := codeToid[b]
	if !ok {
		return NullID
	}
	return v
}

// Returns the byte code asocciated to an ID
func IDToCode(i ID) byte {
	v, ok := idToCode[i]
	if !ok {
		return 0x0
	}
	return v
}

// ERROR CODES

// Determines a generic undefined error
var ErrorUndefined error = errors.New("Undefined problem")

// Invalid operation performed
var ErrorInvalid error = errors.New("Invalid operation performed")

// Content could not be found
var ErrorNotFound error = errors.New("Content not found")

// Versions do not match
var ErrorVersion error = errors.New("Versions do not match")

// Verification handshake failed
var ErrorHandshake error = errors.New("Handshake failed")

// Invalid arguments given
var ErrorArguments error = errors.New("Invalid arguments")

// Payload size too big
var ErrorMaxSize error = errors.New("Payload size too big")

// User is not logged in
var ErrorNoSession error = errors.New("User is not connected")

var errorCodes map[error]byte = map[error]byte{
	ErrorUndefined: 0x00,
	ErrorInvalid:   0x01,
	ErrorNotFound:  0x02,
	ErrorVersion:   0x03,
	ErrorHandshake: 0x04,
}

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
