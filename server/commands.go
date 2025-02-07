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
		//* Username too big
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Assign public key
	key, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		log.Println(err)
		//* Incorrect with public key
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}
	u.pubkey = key

	// Register user into the database
	insertUser(h.db, u.name, cmd.Args[1])
}

func connectUser(h *Hub, u *User, cmd gc.Command) {
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
	h.verifs[u.conn] = &Verif{
		name: u.name,
		text: string(ran),
	}
	h.vmut.Unlock()

	// Wait timeout and remove the entry
	go func() {
		w := time.Duration(gc.LoginTimeout)
		time.Sleep(w * time.Minute)
		h.vmut.Lock()
		delete(h.verifs, u.conn)
		h.vmut.Unlock()
	}()

}

func verifyUser(h *Hub, u *User, cmd gc.Command) {
	// Get the text to verify
	h.vmut.Lock()
	verif, ok := h.verifs[u.conn]
	h.vmut.Unlock()

	// Check if the user is in verification
	if !ok {
		//! This shouldnt happen as its checked by the hub first
		log.Printf("%s is not in verification!\n", u.name)
		//* User is not being verified
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Check if the text is correct
	if verif.text != string(cmd.Args[1]) || verif.name != u.name {
		log.Printf("%s verification is incorrect!\n", u.name)
		//* Incorrect decyphered text
		sendErrorPacket(cmd.HD.ID, gc.ErrorHandshake, u.conn)
		return
	}

	// Everything went fine so we cache the user
	h.umut.Lock()
	h.users[u.conn] = u
	h.umut.Unlock()

	// We delete the pending verification
	h.vmut.Lock()
	delete(h.verifs, u.conn)
	h.vmut.Unlock()
}

func disconnectUser(h *Hub, u *User, cmd gc.Command) {
	// See if the user that wants to disconnect is even connected
	h.umut.Lock()
	_, uok := h.users[u.conn]
	h.umut.Unlock()

	// See if its in verification
	h.vmut.Lock()
	_, vok := h.verifs[u.conn]
	h.vmut.Unlock()

	// If user is in none of the caches we error
	if !uok && !vok {
		log.Printf("Invalid operation performed!")
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Othersie we cleanup
	h.cleanupConn(u.conn)
}
