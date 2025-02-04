package test

import (
	"net"
	"testing"

	. "github.com/Sprinter05/gochat/gcspec"
)

func setup(t *testing.T) net.Conn {
	l, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestPacket(t *testing.T) {
	l := setup(t)
	defer l.Close()

	// OK Packet with 2 arguments
	p := []Arg{Arg("Hello this is a test\nDoes it work?"), Arg("I sure hope so")}
	test1, err := NewPacket(REG, EmptyInfo, p)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test1)

	// OK Packet with 1 argument
	test2, err := NewPacket(MSG, EmptyInfo, []Arg{Arg("Hello there!")})
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test2)

	// OK Packet with no arguments
	test3, err := NewPacket(USRS, EmptyInfo, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test3)

	// OK Packet with error code
	test4, err := NewPacket(ERR, ErrorCode(ErrorHandshake), nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test4)
}
