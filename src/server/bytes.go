package main

import (
	"encoding/binary"
)

type Header struct {
	version uint8
	action  uint8
	info    uint8
}

func versionBitMask(v uint16) uint8 { return uint8(v >> 13) }
func actionBitMask(v uint16) uint8  { return uint8(v>>6) &^ 0b11100000 }
func infoBitMask(v uint16) uint8    { return uint8(v) &^ 0b11000000 }

func tokenizeHeader(header [2]byte) Header {
	h := binary.BigEndian.Uint16(header[:])
	return Header{versionBitMask(h), actionBitMask(h), infoBitMask(h)}
}
