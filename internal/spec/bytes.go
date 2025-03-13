// This package implements several different functions and types
// that are common to both the client and the server, and
// strictly follows the protocol specification.
//
// Please refer to the [Implementation] and the [Specification] for more information:
//
// [Implementation]: https://github.com/Sprinter05/gochat/tree/main/doc/IMPLEMENTATION.md
// [Specification]: https://github.com/Sprinter05/gochat/tree/main/doc/SPECIFICATION.md
package spec

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

/* TYPES */

// Identifies the header of a packet
// split into its fields depending on the
// bit size of each field.
type Header struct {
	Ver  uint8  // Protocol version
	Op   Action // Operation to be performed
	Info uint8  // Additional pacjet information
	Args uint8  // Amount of arguments
	Len  uint16 // Total length of all arguments
	ID   ID     // Packet identifier
}

// Specifies the identifier of the packet that has been sent.
type ID uint16

// Specifies a command together with header and arguments.
type Command struct {
	HD   Header   // Packet header
	Args [][]byte // Packet arguments
}

/* COMMAND FUNCTIONS */

// Prints to standard output all information about a packet.
func (cmd *Command) Print() {
	fmt.Println("-------- HEADER --------")
	fmt.Printf("* Version: %d\n", cmd.HD.Ver)
	fmt.Printf("* Action: %d (%s)\n", cmd.HD.Op, CodeToString(cmd.HD.Op))
	fmt.Printf("* Info: %d\n", cmd.HD.Info)
	if cmd.HD.Op == ERR {
		fmt.Printf("* Error: %s\n", ErrorCodeToError(cmd.HD.Info))
	}
	if cmd.HD.Op == ADMIN {
		fmt.Printf("* Admin: %s\n", AdminString(cmd.HD.Info))
	}
	fmt.Printf("* Args: %d\n", cmd.HD.Args)
	fmt.Printf("* Length: %d\n", cmd.HD.Len)
	fmt.Printf("* ID: %d\n", cmd.HD.ID)
	fmt.Println("-------- PAYLOAD --------")
	for i, v := range cmd.Args {
		fmt.Printf("[%d] %s\n", i, v)
	}
	fmt.Println()
}

// TODO: Unnecessary?
// Prints summarized information about a packet for the client shell.
func (cmd *Command) ShellPrint() {
	// Initializes information message to EmptyInfo message
	inf := "No information"
	// If the information is an error, sets the information message to the error's
	if cmd.HD.Info != 0xFF {
		inf = ErrorCodeToError(cmd.HD.Info).Error()
	}
	// Prints header information
	fmt.Printf("Packet with ID %x (%s) received with information code %x (%s)", IDToCode(cmd.HD.Op), CodeToString(cmd.HD.Op), cmd.HD.Info, inf)
	// Checks argument count
	if len(cmd.Args) == 0 {
		fmt.Printf(". No arguments.\n")
	} else {
		// Prints arguments
		fmt.Printf("\nArguments: ")
		for i, v := range cmd.Args {
			fmt.Printf("Arg %d: %s ", i, v)
		}
		fmt.Print(".\n")
	}
}

/* HEADER FUNCTIONS */

// Checks the validity of the header fields
// Follows the specification of commands sent to the server.
func (hd Header) ServerCheck() error {
	if hd.Ver != ProtocolVersion {
		return ErrorVersion
	}

	if hd.Op == NullOp {
		return ErrorHeader
	}

	// These operations cannot accept empty info field
	check := hd.Op == USRS || hd.Op == ADMIN || hd.Op == ERR
	if check && hd.Info == EmptyInfo {
		return ErrorHeader
	}

	// No commands sent to server can have a null ID
	if hd.ID == NullID {
		return ErrorHeader
	}

	if int(hd.Args) < ServerArgs(hd.Op) {
		return ErrorHeader
	}

	return nil
}

// Checks the validity of the header fields
// Follows the specification of commands sent to the client.
func (hd Header) ClientCheck() error {
	if hd.Ver != ProtocolVersion {
		return ErrorVersion
	}

	if hd.Op == NullOp {
		return ErrorHeader
	}

	// Only these operations can have a null ID
	check := hd.Op == SHTDWN || hd.Op == RECIV || hd.Op == OK
	if !check && hd.ID == NullID {
		return ErrorHeader
	}

	if int(hd.Args) < ClientArgs(hd.Op) {
		return ErrorHeader
	}

	return nil
}

// Splits a byte slice into the fields of a header.
func NewHeader(hdr []byte) Header {
	h := binary.BigEndian.Uint64(hdr[:HeaderSize])
	return Header{
		Ver:  uint8(h >> 60),
		Op:   CodeToID(uint8(h >> 52)),
		Info: uint8(h >> 44),
		Args: (uint8(h >> 40)) &^ 0xF0,        // 0b1111_0000
		Len:  (uint16(h >> 26)) &^ 0xC000,     // 0b1100_0000_0000_0000
		ID:   ID((uint16(h >> 16)) &^ 0xFC00), // 0b1111_1100_0000_0000
	}
}

/* UNIX STAMP FUNCTIONS */

// Turns a time type into its unix timestamp
// as a byte slice, following the size specified
// by the Unix() function of the [time] package.
func UnixStampToBytes(s time.Time) []byte {
	unix := s.Unix()
	p := make([]byte, 0, binary.Size(unix))
	p = binary.AppendVarint(p, unix)
	return p
}

// Turns a byte slice into a time type by reading it as
// a unix timestamp, according to the size specified by
// the [time] package.
func BytesToUnixStamp(b []byte) (t time.Time, e error) {
	min := binary.Size(t.Unix())
	if len(b) < min {
		return t, ErrorArguments
	}

	buf := bytes.NewBuffer(b[:min])
	stamp, err := binary.ReadVarint(buf)
	if err != nil {
		return t, ErrorArguments
	}

	return time.Unix(stamp, 0), nil
}

/* PACKET FUNCTIONS */

// Creates a packet ready to be sent through a TCP connection with all header fields,
// arguments, and delimiters. Arguments are optional and an error will be returned if
// any of the function parameters are malformed.
func NewPacket(op Action, id ID, inf byte, arg ...[]byte) ([]byte, error) {
	l := len(arg)
	if l > MaxArgs {
		return nil, ErrorArguments
	}

	// Check that the ID is not over the bit field size
	if id > MaxID {
		return nil, ErrorArguments
	}

	// Check total payload size
	tot := 0
	if l != 0 {
		for _, v := range arg {
			le := len(v) + 2 // CRLF is 2 bytes
			// Overflows single argument size
			if le > MaxArgSize {
				return nil, ErrorMaxSize
			}
			tot += le
		}
		if tot > MaxPayload {
			return nil, ErrorMaxSize
		}
	}

	// Allocate enough space for the packet
	// Allocates an extra 2 bytes for the header separator
	p := make([]byte, 0, HeaderSize+tot+2)

	// Set all header bits
	b := (uint64(ProtocolVersion) << 60) |
		(uint64(IDToCode(op)) << 52) |
		(uint64(inf) << 44) |
		(uint64(l) << 40) |
		(uint64(tot) << 26) |
		(uint64(id) << 16) |
		0xFFFF // Reserved (not in use)

	// Append header
	p = binary.BigEndian.AppendUint64(p, b)

	// CRLF termination
	p = append(p, "\r\n"...)

	// Append payload arguments
	for _, v := range arg {
		p = append(p, v...)
		p = append(p, "\r\n"...)
	}

	return p, nil
}

/* CRYPTO FUNCTIONS */

// Turns an RSA private key into a PEM byte array
// using the PKCS1 format.
func PrivkeytoPEM(privkey *rsa.PrivateKey) []byte {
	b := x509.MarshalPKCS1PrivateKey(privkey)

	p := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: b,
		},
	)

	return p
}

// Turn an RSA public key into a PEM byte array
// using the PKIX format.
func PubkeytoPEM(pubkey *rsa.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		return nil, err
	}

	p := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: b,
		},
	)

	return p, nil
}

// Gets the private RSA key from a PEM byte array
// using the PKCS1 format.
func PEMToPrivkey(privPEM []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		return nil, errors.New("PEM parsing failed")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return priv, nil
}

// Gets the public RSA key from a PEM byte array
// using the PKIX format.
func PEMToPubkey(pubPEM []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pubPEM)
	if block == nil {
		return nil, errors.New("PEM parsing failed")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// Check if its a public key
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		return pub, nil
	default:
		break // Fall through
	}

	return nil, errors.New("key type is not RSA")
}

// Encrypts a text using a public key and the OAEP method with SHA256.
func EncryptText(t []byte, pub *rsa.PublicKey) ([]byte, error) {
	// Cypher the payload
	hash := sha256.New()
	enc, err := rsa.EncryptOAEP(hash, rand.Reader, pub, t, nil)
	if err != nil {
		return nil, err
	}
	return enc, nil
}

// Decrypts a cyphertext using a private key and the OAEP method with SHA256.
func DecryptText(e []byte, priv *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()
	dec, err := rsa.DecryptOAEP(hash, rand.Reader, priv, e, nil)
	if err != nil {
		return nil, err
	}
	return dec, nil
}
