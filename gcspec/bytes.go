package gcspec

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
)

/* TYPES */

// Identifies a header split into its fields as single bytes
type Header struct {
	Ver  uint8
	ID   ID
	Info uint8
	Args uint8
	Len  uint16
}

// Used to specify its coming from a command
type Arg []byte

// Specifies a command
type Command struct {
	HD   Header
	Args []Arg
}

/* COMMAND FUNCTIONS */

// Prints all information about a packet
func (c Command) Print() {
	fmt.Println("**HEADER:**")
	fmt.Printf("Version: %d\n", c.HD.Ver)
	fmt.Printf("Action ID: %d\n", c.HD.ID)
	fmt.Printf("Info: %d\n", c.HD.Info)
	fmt.Printf("Arguments: %d\n", c.HD.Args)
	fmt.Printf("Payload Length: %d\n", c.HD.Len)
	fmt.Println("**PAYLOAD:**")
	for i, v := range c.Args {
		fmt.Printf("Arg %d: %s\n", i, v)
	}
	fmt.Println()
}

/* HEADER FUNCTIONS */

// Checks the validity of the header fields
func (hd Header) Check() error {
	if hd.Ver != ProtocolVersion {
		return ErrorVersion
	}

	if hd.ID == NullID {
		return ErrorInvalid
	}

	return nil
}

// Splits a the byte header into its fields
func NewHeader(hdr []byte) Header {
	h := binary.BigEndian.Uint32(hdr[:HeaderSize])
	return Header{
		Ver:  uint8(h >> 28),
		ID:   CodeToID(uint8(h >> 20)),
		Info: uint8(h >> 12),
		Args: (uint8(h >> 10)) &^ 0xFC,
		Len:  uint16(h) &^ 0xFC00,
	}
}

/* PACKET FUNCTIONS */

// Creates a byte slice corresponding to the header fields
// This function only checks size bounds not argument integrityy
// like containg CRLF at the end of each argument
func NewPacket(id ID, inf byte, arg []Arg) ([]byte, error) {
	// Verify number of arguments
	l := len(arg)
	if l > MaxArgs {
		return nil, ErrorArguments
	}

	// Check total payload size
	tot := 0
	if l != 0 {
		for _, v := range arg {
			tot += len(v) + 2 // CRLF is 2 bytes
		}
		if tot > MaxPayload {
			return nil, ErrorMaxSize
		}
	}

	// Allocate enough space for the packet
	// Allocates an extra 2 bytes for the header separator
	p := make([]byte, 0, HeaderSize+tot+2)

	// Set all header bits
	b := (uint32(ProtocolVersion) << 28) |
		(uint32(IDToCode(id)) << 20) |
		(uint32(inf) << 12) |
		(uint32(l) << 10) |
		(uint32(tot))

	// Append header to slice
	p = binary.BigEndian.AppendUint32(p, b)
	p = append(p, "\r\n"...)

	// Append payload arguments
	for _, v := range arg {
		p = append(p, v...)
		p = append(p, "\r\n"...)
	}

	return p, nil
}

/* CRYPTO FUNCTIONS */

// Turns an RSA private key to a PEM byte array
// Uses the PKCS1 format
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

// Turn an RSA public key to a PEM byte array
// Uses the PKIX format
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
// Uses the PKCS1 format
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
// Uses the PKIX format
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

	return nil, errors.New("Key type is not RSA")
}

// Encrypts a text using OAEP with SHA256
func EncryptText(t []byte, pub *rsa.PublicKey) ([]byte, error) {
	// Cypher the payload
	hash := sha256.New()
	enc, err := rsa.EncryptOAEP(hash, rand.Reader, pub, t, nil)
	if err != nil {
		return nil, errors.New("Impossible to encrypt")
	}
	return enc, nil
}

// Decrypts a cyphertext using OAEP with SHA256
func DecryptText(e []byte, priv *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.New()
	dec, err := rsa.DecryptOAEP(hash, rand.Reader, priv, e, nil)
	if err != nil {
		return nil, errors.New("Impossible to decrypt")
	}
	return dec, nil
}
