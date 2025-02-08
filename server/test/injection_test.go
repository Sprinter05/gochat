package test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"testing"

	gc "github.com/Sprinter05/gochat/gcspec"
)

func setup(t *testing.T) net.Conn {
	l, err := net.Dial("tcp4", "127.0.0.1:6969")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func readFromConn(c *gc.Connection) gc.Command {
	cmd := new(gc.Command)
	c.ListenHeader(cmd)
	c.ListenPayload(cmd)
	return *cmd
}

/*func readKeyFile(name string) []byte {
	file, _ := os.Open(name)
	defer file.Close()

	// Get the file size
	stat, _ := file.Stat()

	// Read the file into a byte slice
	bs := make([]byte, stat.Size())
	bufio.NewReader(file).Read(bs)

	return bs
}*/

func TestREG(t *testing.T) {
	l := setup(t)
	defer l.Close()

	//pub := readKeyFile("public.pem")
	//priv := readKeyFile("private.pem")
	//dekey, _ := gc.PEMToPrivkey(priv)

	conn := &gc.Connection{
		Conn: l,
		RD:   bufio.NewReader(l),
	}

	// Create rsa key
	v, _ := rsa.GenerateKey(rand.Reader, gc.RSABitSize)
	b, _ := gc.PubkeytoPEM(&v.PublicKey)

	// REG Packet
	p1 := []gc.Arg{gc.Arg("Sprinter05"), gc.Arg(b)}
	test1, err := gc.NewPacket(gc.REG, gc.ID(19736), gc.EmptyInfo, p1)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test1)

	readFromConn(conn) // ignored OK

	// Login
	p2 := []gc.Arg{gc.Arg("Sprinter05")}
	test2, err := gc.NewPacket(gc.CONN, gc.ID(8945), gc.EmptyInfo, p2)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test2)

	readFromConn(conn)         // ignored OK
	vpak := readFromConn(conn) // VERIF packet

	dec, _ := gc.DecryptText(vpak.Args[0], v)

	// Verify
	p3 := []gc.Arg{gc.Arg("Sprinter05"), gc.Arg(string(dec))}
	test3, err := gc.NewPacket(gc.VERIF, gc.ID(11333), gc.EmptyInfo, p3)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test3)
}
