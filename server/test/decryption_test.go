package test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/Sprinter05/gochat/internal/spec"
)

func TestEncdec(t *testing.T) {
	// Create key
	key, err := rsa.GenerateKey(rand.Reader, spec.RSABitSize)
	if err != nil {
		t.Fatal(err)
	}

	// Go through PEM array
	pempub, err := spec.PubkeytoPEM(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pempriv := spec.PrivkeytoPEM(key)

	// Turn back to key
	pub, err := spec.PEMToPubkey(pempub)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := spec.PEMToPrivkey(pempriv)
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt text
	text := "Man this is so cumbersome"
	enc, err := spec.EncryptText(
		[]byte(text),
		pub,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Decrypt text
	dec, err := spec.DecryptText(enc, priv)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("BEFORE: %s\n\n", text)
	t.Logf("ENCRYPTED: %s\n\n", enc)
	t.Logf("AFTER: %s\n\n", dec)
}
