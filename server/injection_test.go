package main

import (
	"net"
	"testing"
)

func TestPacketInjection(t *testing.T) {
	l, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		t.Fatal(err)
	}

	p := []byte("Hello this is a test\nDoes it work?\r\nI sure hope so\r\n")

	hd := []byte{0b00010000, 0b00111111, 0b11111000}
	hd = append(hd, byte(len(p)))
	hd = append(hd, "\r\n"...)
	sent := append(hd, p...)

	l.Write(sent)
	l.Close()
}
