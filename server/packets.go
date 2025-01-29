package main

import (
	"github.com/Sprinter05/gochat/gcspec"
)

type Packet struct {
	hdr     gcspec.Header
	len     gcspec.Length
	payload []byte
}
