package gcspec

import (
	"encoding/binary"
)

// Identifies a header split into its fields as single bytes
type Header struct {
	Version uint8
	Action  uint8
	Info    uint8
	Args    uint8
	Length  uint16
}

type Command struct {
	Header    Header
	Arguments []string
}

// Splits a the byte header into its fields
func NewHeader(hdr []byte) Header {
	h := binary.BigEndian.Uint32(hdr[:HeaderSize])
	return Header{
		Version: uint8(h >> 28),
		Action:  uint8(h >> 20),
		Info:    uint8(h >> 12),
		Args:    (uint8(h >> 10)) &^ 0xFC,
		Length:  uint16(h) &^ 0xFC00,
	}
}

// Creates a byte slice corresponding to the header fields
// This function only checks size bounds not argument integrityy
// like containg CRLF at the end of each argument
func NewPacket(id ID, inf byte, arg []string) ([]byte, error) {
	// Verify number of arguments
	l := len(arg)
	if l > MaxArgs {
		return nil, ErrorArguments
	}

	// Check total payload size
	tot := 0
	if l != 0 {
		for _, v := range arg {
			tot += len(v)
		}
		if tot > MaxPayload {
			return nil, ErrorMaxSize
		}
	}

	// Allocate enough space for the packet
	// Allocates an extra 2 bytes for the header separator
	p := make([]byte, 0, HeaderSize+tot+2)

	// Set all header bits
	b := (uint32(ProtocolVersion) << 28) |
		(uint32(IDToCode(id)) << 20) |
		(uint32(inf) << 12) |
		(uint32(l) << 10) |
		(uint32(tot))

	// Append header to slice
	p = binary.BigEndian.AppendUint32(p, b)
	p = append(p, "\r\n"...)

	// Append payload arguments
	for _, v := range arg {
		p = append(p, v...)
	}

	return p, nil
}
