package protocol

import (
	"errors"
)

// VERSION

const Version = 1

// FIXED SIZES

const HeaderSize int = 2 // bytes
const LengthSize int = 2 // bytes

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

func GetServerActionCode(i uint8) string {
	v, ok := serverActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

func GetClientActionCode(i uint8) string {
	v, ok := clientActionCodes[i]
	if !ok {
		return ""
	}
	return v
}

// ERROR CODES

var ErrorUndefined error = errors.New("ERR_UNDEFINED")
var ErrorInvalid error = errors.New("ERR_INVALID")
var ErrorVersion error = errors.New("ERR_VERSION")
var ErrorNoConnection error = errors.New("ERR_NOCONN")
var ErrorNotFound error = errors.New("ERR_NOTFOUND")
var ErrorHandshake error = errors.New("ERR_HANDSHAKE")
var ErrorBrokenMsg error = errors.New("ERR_BROKENMSG")
