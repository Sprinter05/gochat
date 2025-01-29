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

// Specifies the size of the length
type Length uint16

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
func NewLength(len []byte) Length {
	return Length(binary.BigEndian.Uint16(len[:LengthSize]))
}

// Returns a byte array with the fields of the header
func NewByteHeader(hdr Header) []byte {
	p := (uint16(hdr.Version) << 13) | (uint16(hdr.Action) << 6) | uint16(hdr.Info)
	var b [HeaderSize]byte
	binary.BigEndian.PutUint16(b[:], p)
	return b[:]
}

// Returns a byte array with the length of the payload
func NewByteLength(len Length) []byte {
	var b [LengthSize]byte
	binary.BigEndian.PutUint16(b[:], uint16(len))
	return b[:]
}
