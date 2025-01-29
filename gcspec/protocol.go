package gcspec

import (
	"errors"
)

// Version of the protocol being used
const SpecVersion = 1

// Size of the header in bytes
const HeaderSize int = 2

// Size of the payload length in bytes
const LengthSize int = 2

// Empty information field
const EmptyInfo uint8 = 0b00111111

// ACTION CODES

var serverActionCodes map[uint8]string = map[uint8]string{
	0x01: "OK",
	0x02: "ERR",
	0x03: "SHTDWN",
	0x04: "VERIF",
	0x05: "REQ",
	0x06: "USRS",
	0x07: "RECIV",
}

var clientActionCodes map[uint8]string = map[uint8]string{
	0x01: "OK",
	0x02: "ERR",
	0x03: "REG",
	0x04: "VERIF",
	0x05: "REQ",
	0x06: "USRS",
	0x07: "CONN",
	0x08: "MSG",
	0x09: "DISCN",
	0x0A: "DEREG",
}

// Returns the operation string code or an empty string if it does not exist
func ServerActionCode(i uint8) string {
	v, ok := serverActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

// Returns the operation string code or an empty string if it does not exist
func ClientActionCode(i uint8) string {
	v, ok := clientActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

// ERROR CODES

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

var errorCodes map[error]uint8 = map[error]uint8{
	ErrorUndefined: 0x00,
	ErrorInvalid:   0x01,
	ErrorNotFound:  0x02,
	ErrorVersion:   0x03,
	ErrorHandshake: 0x04,
}

// Returns the error code or the empty information field if not found
func ErrorCode(err error) uint8 {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
