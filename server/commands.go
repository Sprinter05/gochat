package main

import (
	"context"
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
		// Username too big
		//log.Printf("Supplied username %s is too big\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Assign public key
	key, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		// Incorrect with public key
		//log.Printf("Incorrect public key from %s when registering: %s\n", u.name, err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}
	u.pubkey = key

	// Register user into the database
	insertUser(h.db, u.name, cmd.Args[1])
	sendOKPacket(cmd.HD.ID, u.conn)
}

func connectUser(h *Hub, u *User, cmd gc.Command) {
	// Create random cypher
	ran := randText()
	enc, err := gc.EncryptText(ran, u.pubkey)
	if err != nil {
		// Error with cyphering
		//! This shouldnt happen, it means the database for the user is corrupted
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		log.Fatalf("%s has inconsistent database publickey: %s!\n", u.name, err)
		return
	}

	// We create and send the packet with the enconded text
	arg := []gc.Arg{
		gc.Arg(enc),
	}
	vpak, e := gc.NewPacket(gc.VERIF, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating VERIF packet: %s\n", e)
		return
	}
	u.conn.Write(vpak)

	// Context used for goroutine
	ctx, cancl := context.WithCancel(context.Background())

	// Add the user to the pending verifications
	ins := &Verif{
		name:   u.name,
		text:   string(ran),
		cancel: cancl,
	}
	h.verifs.Add(u.conn, ins)

	// Wait timeout and remove the entry
	// This function is a closure
	go func() {
		w := time.Duration(gc.LoginTimeout) * time.Minute
		select {
		case <-time.After(w):
			// Verification timeout
			h.verifs.Remove(u.conn)
		case <-ctx.Done():
			// Verification complete
			return
		}
	}()
}

func verifyUser(h *Hub, u *User, cmd gc.Command) {
	// Get the text to verify
	verif, ok := h.verifs.Get(u.conn)

	// Check if the user is in verification
	if !ok {
		// User is not being verified
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		log.Printf("%s is not in verification!\n", u.name)
		return
	}

	// Check if the text is correct
	if verif.text != string(cmd.Args[1]) || verif.name != u.name {
		//log.Printf("%s verification is incorrect\n", u.name)
		// We cancel the goroutine and remove the verification
		verif.cancel()
		h.cleanupUser(u.conn)
		sendErrorPacket(cmd.HD.ID, gc.ErrorHandshake, u.conn)
		return
	}

	// Everything went fine so we cache the user
	h.users.Add(u.conn, u)

	// We delete the pending verification and cancel the goroutine
	verif.cancel()
	h.users.Remove(u.conn)

	// Perform catchup for the logged in user after acknowledge
	sendOKPacket(cmd.HD.ID, u.conn)
	h.wrapCatchUp(u)
}

func disconnectUser(h *Hub, u *User, cmd gc.Command) {
	// See if the user that wants to disconnect is even connected
	_, uok := h.users.Get(u.conn)

	// See if its in verification
	_, vok := h.verifs.Get(u.conn)

	// If user is in none of the caches we error
	if !uok && !vok {
		// Error since the user is not connected
		//log.Printf("%s trying to disconnect when not connected\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	// Otherwise we cleanup
	h.cleanupUser(u.conn)
	sendOKPacket(cmd.HD.ID, u.conn)
}

func deregisterUser(h *Hub, u *User, cmd gc.Command) {
	// Cleanup cache information in any case
	defer h.cleanupUser(u.conn)

	// Delete if message cache is empty
	e := removeUser(h.db, u.name)
	if e == nil {
		// User deleted, everything worked
		return
	}

	// Undefined problem
	if e != ErrorDBConstraint {
		// Error when deleting that is not the constraint
		//! This means a problem with the database occured
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		log.Fatalf("Undefined database error when deleting %s: %s!\n", u.name, e)
		return
	}

	// The user has cached messages
	// Attempt to remove the key from the user
	err := removeKey(h.db, u.name)
	if err != nil {
		// Error with deleting user key
		//! This should never happen when deleting this key
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		log.Fatalf("Impossible to deregister user %s: %s!\n", u.name, err)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

func requestUser(h *Hub, u *User, cmd gc.Command) {
	// We query the user's key
	k, err := queryUserKey(h.db, username(cmd.Args[0]))
	if err != nil {
		// Error since the key couldnt be found
		log.Printf("Requested user can not be queried: %s\n", err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}

	// Turn the key into PEM format
	p, e := gc.PubkeytoPEM(k)
	if e != nil {
		// Failed to transform the public key
		//! This means the user's database is corrupted info
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		log.Fatalf("%s has inconsistent database publickey: %s!\n", u.name, err)
		return
	}

	// Otherwise we send the key to the user
	arg := []gc.Arg{
		gc.Arg(p),
	}
	pak, e := gc.NewPacket(gc.REQ, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating REQ packet: %s\n", e)
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
		// Error due to invalid argument in header info
		//log.Printf("Invalid user list argument from %s\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// No users found
	if usrs == "" {
		// Could not find any users matching
		//log.Printf("No users exist when querying userlist\n")
		sendErrorPacket(cmd.HD.ID, gc.ErrorEmpty, u.conn)
		return
	}

	// We send the userlist
	arg := []gc.Arg{
		gc.Arg(usrs),
	}
	pak, e := gc.NewPacket(gc.USRS, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating USRS packet: %s\n", e)
		return
	}
	u.conn.Write(pak)
}

func messageUser(h *Hub, u *User, cmd gc.Command) {
	// Find information about the user
	send, ok := h.findUser(username(cmd.Args[0]))
	if ok {
		// We send the message directly to the connection
		// Only the user changes as we keep the same cyphertext
		arg := []gc.Arg{
			gc.Arg(u.name),
			gc.Arg(gc.UnixStampNow()),
			cmd.Args[2],
		}
		pak, e := gc.NewPacket(gc.RECIV, cmd.HD.ID, gc.EmptyInfo, arg)
		if e != nil {
			log.Printf("Error when creating RECIV packet: %s\n", e)
			return
		}
		send.conn.Write(pak)
		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Otherwise we just send it to the message cache
	uname := username(cmd.Args[0])
	err := cacheMessage(h.db, u.name, uname, string(cmd.Args[2]))
	if err != nil {
		// Error when inserting the message into the cache
		log.Printf("Error when caching a message from %s\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}
	sendOKPacket(cmd.HD.ID, u.conn)
}
