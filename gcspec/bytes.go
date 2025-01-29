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
func NewHeader(header []byte) Header {
	h := binary.BigEndian.Uint16(header[:HeaderSize])
	return Header{versionBitMask(h), actionBitMask(h), infoBitMask(h)}
}

// Returns the size in bytes corresponding to the payload
func NewLength(length []byte) Length {
	return Length(binary.BigEndian.Uint16(length[:LengthSize]))
}

// Returns a byte array with the fields of the header
func NewByteHeader(hdr Header) []byte {
	p := (uint16(hdr.Version) << 13) | (uint16(hdr.Action) << 6) | uint16(hdr.Info)
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, p)
	return b
}
