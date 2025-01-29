package gcspec

import (
	"encoding/binary"
)

// Identifies a header split into its fields as single bytes
type Header struct {
	Version uint8
	Action  uint8
	Info    uint8
}

func versionBits(v uint16) uint8 {
	return uint8(v >> 13)
}

func actionBits(v uint16) uint8 {
	return (uint8(v >> 6)) &^ 0b11100000
}

func infoBits(v uint16) uint8 {
	return uint8(v) &^ 0b11000000
}

// Splits a the byte header into its fields
func NewHeader(hdr []byte) Header {
	h := binary.BigEndian.Uint16(hdr[:HeaderSize])
	return Header{versionBits(h), actionBits(h), infoBits(h)}
}

// Returns the size in bytes corresponding to the payload
func NewLength(len []byte) uint16 {
	return binary.BigEndian.Uint16(len[:LengthSize])
}
