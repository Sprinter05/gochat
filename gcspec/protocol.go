package gcspec

import (
	"errors"
)

// Version of the protocol being used
const SpecVersion = 1

// Size of the header in bytes
const HeaderSize int = 4

// Empty information field
const EmptyInfo byte = 0xFF

// ACTION CODES

// Returns the ID associated to a byte code
func CodeToID(b byte) ID {
	v, ok := codeToid[b]
	if !ok {
		return null
	}
	return v
}

// Returns the byte code asocciated to an ID
func IDToCOde(i ID) byte {
	v, ok := idToCode[i]
	if !ok {
		return 0x0
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

// Returns the error code or the empty information field if not found
func ErrorCode(err error) byte {
	v, ok := errorCodes[err]
	if !ok {
		return EmptyInfo
	}
	return v
}
