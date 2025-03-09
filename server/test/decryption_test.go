package test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	gc "github.com/Sprinter05/gochat/gcspec"
)

func TestEncdec(t *testing.T) {
	// Create key
	key, err := rsa.GenerateKey(rand.Reader, gc.RSABitSize)
	if err != nil {
		t.Fatal(err)
	}

	// Go through PEM array
	pempub, err := gc.PubkeytoPEM(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pempriv := gc.PrivkeytoPEM(key)

	// Turn back to key
	pub, err := gc.PEMToPubkey(pempub)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := gc.PEMToPrivkey(pempriv)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt text
	text := "Man this is so cumbersome"
	enc, err := gc.EncryptText(
		[]byte(text),
		pub,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt text
	dec, err := gc.DecryptText(enc, priv)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("BEFORE: %s\n\n", text)
	t.Logf("ENCRYPTED: %s\n\n", enc)
	t.Logf("AFTER: %s\n\n", dec)
}
