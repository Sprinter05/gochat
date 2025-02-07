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
		//* Username too big
		log.Println("Username too big")
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Assign public key
	key, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		//* Incorrect with public key
		log.Println(err)
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
		//* User is not being verified
		log.Printf("%s is not in verification!\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Check if the text is correct
	if verif.text != string(cmd.Args[1]) || verif.name != u.name {
		//* Incorrect decyphered text
		log.Printf("%s verification is incorrect!\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorHandshake, u.conn)
		return
	}

	// Everything went fine so we cache the user
	h.umut.Lock()
	h.users[u.conn] = u
	h.umut.Unlock()

	// TODO: RECIVs should be handled here now in another thread

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
		//* Error since the user is not connected
		log.Printf("Invalid operation performed!")
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Othersie we cleanup
	h.cleanupConn(u.conn)
}

func deregisterUser(h *Hub, u *User, cmd gc.Command) {
	// Attempt to remove the key from the user
	// TODO: The entry should be removed during a catch up if the message cache is empty
	err := removeKey(h.db, u.name)
	if err != nil {
		//* Error with deleting
		//! This should never happen
		log.Println(err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		return
	}

	// Cleanup cache information
	h.cleanupConn(u.conn)
}

func requestUser(h *Hub, u *User, cmd gc.Command) {
	// We query the user's key
	k, err := queryUserKey(h.db, username(cmd.Args[0]))
	if err != nil {
		//* Error since the key couldnt be found
		log.Println(err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}

	// Check if the user queried is deregistered
	if k == nil {
		//* Error since the queried user has been deregistered
		log.Printf("Queried %s has been deregistered!\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Turn the key into PEM format
	p, e := gc.PubkeytoPEM(k)
	if e != nil {
		//* Failed to transform the public key
		//! This means the user's database is corrupted info
		log.Println(e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Otherwise we send the key to the user
	arg := []gc.Arg{gc.Arg(p)}
	pak, e := gc.NewPacket(gc.REQ, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Println(e)
		return
	}
	u.conn.Write(pak)

}

func listUsers(h *Hub, u *User, cmd gc.Command) {
	var usrs string

	// Show online users or all
	online := cmd.HD.Info

	// Get the user list
	if online == 0x01 {
		usrs = h.userlist(true)
	} else if online == 0x00 {
		usrs = h.userlist(false)
	} else {
		//* Error due to invalid argument in header info
		log.Printf("Invalid user list argument!")
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// We send the userlist
	arg := []gc.Arg{gc.Arg([]byte(usrs))}
	pak, e := gc.NewPacket(gc.USRS, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Println(e)
		return
	}
	u.conn.Write(pak)
}

func messageUser(h *Hub, u *User, cmd gc.Command) {
	// Find information about the user
}
