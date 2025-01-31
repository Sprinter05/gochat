package test

import (
	"net"
	"testing"

	. "github.com/Sprinter05/gochat/gcspec"
)

func TestPacketInjection(t *testing.T) {
	l, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		t.Fatal(err)
	}

	p := []string{"Hello this is a test\nDoes it work?", "I sure hope so"}

	test, err := NewPacket(REG, EmptyInfo, p)
	if err != nil {
		t.Fatal(err)
	}

	l.Write(test)
	l.Close()
}
