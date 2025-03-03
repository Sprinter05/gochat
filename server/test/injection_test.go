package test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"testing"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

func setup(t *testing.T) net.Conn {
	l, err := net.Dial("tcp4", "127.0.0.1:9037")
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
	p1 := []gc.Arg{gc.Arg("pepe"), gc.Arg(b)}
	test1, err := gc.NewPacket(gc.REG, gc.ID(976), gc.EmptyInfo, p1)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test1)

	r1 := readFromConn(conn) // ignored OK
	r1.Print()

	// Login
	p2 := []gc.Arg{gc.Arg("pepe")}
	test2, err := gc.NewPacket(gc.LOGIN, gc.ID(894), gc.EmptyInfo, p2)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test2)

	vpak := readFromConn(conn) // VERIF packet
	vpak.Print()

	dec, e := gc.DecryptText(vpak.Args[0], v)
	if e != nil {
		t.Fatal(e)
	}

	// Verify
	p3 := []gc.Arg{gc.Arg("pepe"), gc.Arg(string(dec))}
	test3, err := gc.NewPacket(gc.VERIF, gc.ID(113), gc.EmptyInfo, p3)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test3)

	r2 := readFromConn(conn) // OK packet
	r2.Print()

	// Msg
	p4 := []gc.Arg{
		gc.Arg("Sprinter05"),
		gc.Arg(gc.UnixStampToBytes(time.Now())),
		gc.Arg("akjdaksjdsalkdjaslkdjsalkdjsalkdsj"),
	}
	test4, err := gc.NewPacket(gc.MSG, gc.ID(69), 0x01, p4)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test4)

	r3 := readFromConn(conn) // OK packet
	r3.Print()

}
