package gcspec

import (
	"encoding/binary"
)

type packet struct {
	hdr     Header
	len     Length
	payload []byte
}

func newBytePacket(p packet) []byte {
	// Allocate enough elements for the slice
	pck := make([]byte, 0, HeaderSize+LengthSize+cap(p.payload))

	// Set all bits for the header
	phdr := (uint16(p.hdr.Version) << 13) | (uint16(p.hdr.Action) << 6) | uint16(p.hdr.Info)

	// Append all elements to the slice
	pck = binary.BigEndian.AppendUint16(pck, uint16(phdr))
	pck = binary.BigEndian.AppendUint16(pck, uint16(p.len))
	pck = append(pck, p.payload...)

	return pck
}
