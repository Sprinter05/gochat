package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// DEFINITIONS

type Request struct {
	cl  net.Conn
	cmd gc.Command
}

// FUNCTIONS

// Get Pubkey from PEM byte array
func pemToPub(pubPEM []byte) (*rsa.PublicKey, error) {
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

// Listens from a client and sends itself trough a channel for the hub to process
func listenConnection(cl *gc.Connection, hub chan<- Request) {
	// Close connection when exiting
	defer cl.Conn.Close()

	for {
		cmd := gc.Command{}

		// Read header from the wire
		if err := cl.ListenHeader(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := gc.NewPacket(gc.ERR, gc.ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.Conn.Write(pak)
			}
			continue
		}

		// Read payload from the wire
		if err := cl.ListenPayload(&cmd); err != nil {
			log.Print(err)
			// Connection closed by client
			if err == io.EOF {
				return
			}
			// Send error packet to client
			pak, e := gc.NewPacket(gc.ERR, gc.ErrorCode(err), nil)
			if e != nil { // Error when creating packet
				log.Print(e)
			} else {
				cl.Conn.Write(pak)
			}
			continue
		}

		// Send OK reply to the client
		pak, err := gc.NewPacket(gc.OK, gc.EmptyInfo, nil)
		if err != nil { // Error when creating packet
			log.Print(err)
		} else {
			cl.Conn.Write(pak)
		}

		// Send command to the hub
		hub <- Request{
			cl:  cl.Conn,
			cmd: cmd,
		}
	}

}
