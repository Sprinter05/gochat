package main

import (
	"log"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

func registerUser(h *Hub, u *User, cmd gc.Command) {
	// Assign parameters to the user
	u.name = username(cmd.Args[0])

	// Check if username size is correct
	if len(u.name) > gc.UsernameSize {
		log.Println("Username too big")
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Assign public key
	key, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		//* Error with public key
		log.Println(err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}
	u.pubkey = key

	// Register user into the database
	insertUser(h.db, u.name, cmd.Args[1])
}

func connUser(h *Hub, u *User, cmd gc.Command) {
	ip := ip(u.conn.RemoteAddr().String())

	// Create random cypher
	ran := randText()
	enc, err := gc.EncryptText(ran, u.pubkey)
	if err != nil {
		//* Error with cyphering
		//! This shouldnt happen, it means the database for the user is corrupted
		log.Println(err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		return
	}

	// We create the packet with the enconded text
	arg := []gc.Arg{gc.Arg(enc)}
	vpak, e := gc.NewPacket(gc.VERIF, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Println(e)
		return
	}

	// Send the encrypted cyphertext
	u.conn.Write(vpak)

	// Add the user to the pending verifications
	h.vmut.Lock()
	h.verifs[ip] = string(ran)
	h.vmut.Unlock()

	// Wait timeout and remove the entry
	w := time.Duration(gc.LoginTimeout)
	time.Sleep(w * time.Second)
	h.vmut.Lock()
	delete(h.verifs, ip)
	h.vmut.Unlock()
}
