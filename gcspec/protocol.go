package gcspec

import (
	"errors"
)

// Version of the protocol being used
const SpecVersion uint = 1

// Size of the header in bytes
const HeaderSize int = 4

// Maximum arguments allowed
const MaxArgs int = 1<<2 - 1

// Maximum payload size (includes CRLFs)
const MaxPayload int = 1<<10 - 1

// ACTION CODES

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

// Invalid argument field
var ErrorArguments error = errors.New("Invalid arguments")

// Payload size too big
var ErrorMaxSize error = errors.New("Payload size too big")

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
