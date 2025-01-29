package gcspec

import (
	"encoding/binary"
	"math"
)

func packetToBytes(hd Header, pl []byte) []byte {
	// Crop to the maximum size
	len := cap(pl)
	if len > math.MaxUint16 {
		len = math.MaxUint16
	}
	payload := pl[:len]

	// Allocate enough elements for the slice
	p := make([]byte, 0, HeaderSize+LengthSize+cap(pl))

	// Set all bits for the header
	hdb := (uint16(hd.Version) << 13) | (uint16(hd.Action) << 6) | uint16(hd.Info)

	// Append all elements to the slice
	p = binary.BigEndian.AppendUint16(p, hdb)
	p = binary.BigEndian.AppendUint16(p, uint16(len))
	p = append(p, payload...)

	return p
}
