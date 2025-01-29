package gcspec

import (
	"encoding/binary"
)

// Identifies a header split into its fields as single bytes
type Header struct {
	Version uint8
	Action  uint8
	Info    uint8
	Flags   uint8
	Length  uint16
}

type arg []byte

type Command struct {
	Header    Header
	Arguments []arg
}

// Splits a the byte header into its fields
func NewHeader(hdr []byte) Header {
	h := binary.BigEndian.Uint32(hdr[:HeaderSize])
	return Header{
		Version: uint8(h >> 28),
		Action:  uint8(h >> 20),
		Info:    uint8(h >> 12),
		Flags:   (uint8(h >> 10)) &^ 0xFC,
		Length:  uint16(h) &^ 0xFC00,
	}
}
