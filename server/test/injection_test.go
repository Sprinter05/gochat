package test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
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
	cmd := &gc.Command{}
	c.ListenHeader(cmd)
	c.ListenPayload(cmd)
	return *cmd
}

func readKeyFile(name string) []byte {
	file, _ := os.Open(name)
	defer file.Close()

	// Get the file size
	stat, _ := file.Stat()

	// Read the file into a byte slice
	bs := make([]byte, stat.Size())
	bufio.NewReader(file).Read(bs)

	return bs
}

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
	p := []gc.Arg{gc.Arg("Sprinter05"), gc.Arg(b)}
	test1, err := gc.NewPacket(gc.REG, gc.ID(19736), gc.EmptyInfo, p)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test1)

	readFromConn(conn)
	p2 := readFromConn(conn)

	dec, _ := gc.DecryptText(p2.Args[0], v)

	t.Log("\n" + string(dec) + "\n")

	return
}
