package protocol

import (
	"errors"
)

// Version of the protocol being used
const Version = 1

// Size of the header in bytes
const HeaderSize int = 2

// Size of the payload length in bytes
const LengthSize int = 2

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
func GetServerActionCode(i uint8) string {
	v, ok := serverActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

// Returns the operation string code or an empty string if it does not exist
func GetClientActionCode(i uint8) string {
	v, ok := clientActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

// ERROR CODES

// Determines a generic undefined error
var ErrorUndefined error = errors.New("ERR_UNDEFINED")

// Invalid operation performed
var ErrorInvalid error = errors.New("ERR_INVALID")

// Versions do not match
var ErrorVersion error = errors.New("ERR_VERSION")

// Content could not be found
var ErrorNotFound error = errors.New("ERR_NOTFOUND")

// Verification handshake failed
var ErrorHandshake error = errors.New("ERR_HANDSHAKE")
