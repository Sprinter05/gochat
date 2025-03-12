package test

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"testing"
	"time"

	"github.com/Sprinter05/gochat/internal/spec"
)

func setup(t *testing.T) *tls.Conn {
	config := &tls.Config{
		InsecureSkipVerify: true,
	}

	l, err := tls.Dial("tcp4", "127.0.0.1:8037", config)
	if err != nil {
		t.Fatal(err)
	}

	return l
}

func readFromConn(c *spec.Connection) spec.Command {
	cmd := new(spec.Command)
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
	//dekey, _ := spec.PEMToPrivkey(priv)

	conn := &spec.Connection{
		Conn: l,
		RD:   bufio.NewReader(l),
	}

	// Initial handshake
	readFromConn(conn) // ignored OK

	// Create rsa key
	v, _ := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	b, _ := spec.PubkeytoPEM(&v.PublicKey)

	// REG Packet
	test1, err := spec.NewPacket(spec.REG, spec.ID(976), spec.EmptyInfo, []byte("pepe"), b)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test1)

	r1 := readFromConn(conn) // ignored OK
	r1.Print()

	// Login
	test2, err := spec.NewPacket(spec.LOGIN, spec.ID(894), spec.EmptyInfo, []byte("pepe"))
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test2)

	vpak := readFromConn(conn) // VERIF packet
	vpak.Print()

	dec, e := spec.DecryptText(vpak.Args[0], v)
	if e != nil {
		t.Fatal(e)
	}

	// Verify
	test3, err := spec.NewPacket(spec.VERIF, spec.ID(113), spec.EmptyInfo, []byte("pepe"), dec)
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test3)

	r2 := readFromConn(conn) // OK packet
	r2.Print()

	// Msg
	test4, err := spec.NewPacket(spec.MSG, spec.ID(69), 0x01, []byte("Sprinter05"), spec.UnixStampToBytes(time.Now()), []byte("hola q tal"))
	if err != nil {
		t.Fatal(err)
	}
	l.Write(test4)

	r3 := readFromConn(conn) // OK packet
	r3.Print()

}
