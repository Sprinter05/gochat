package protocol

import (
	"encoding/binary"
)

// Identifies a header split into its fields as single bytes
type Header struct {
	Version uint8
	Action  uint8
	Info    uint8
}

func versionBitMask(v uint16) uint8 {
	return uint8(v >> 13)
}

func actionBitMask(v uint16) uint8 {
	return (uint8(v >> 6)) &^ 0b11100000
}

func infoBitMask(v uint16) uint8 {
	return uint8(v) &^ 0b11000000
}

// Splits a the byte header into its fields
func GetHeader(header []byte) Header {
	h := binary.BigEndian.Uint16(header[:HeaderSize])
	return Header{versionBitMask(h), actionBitMask(h), infoBitMask(h)}
}

// Returns the size in bytes corresponding to the payload
func GetLength(length []byte) uint16 {
	return binary.BigEndian.Uint16(length[:LengthSize])
}
