package main

import (
	"context"
	"log"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// FUNCTIONS

// Replies with OK or ERR
// Uses a user with only the net.Conn
func registerUser(h *Hub, u User, cmd gc.Command) {
	uname := username(cmd.Args[0])

	if len(uname) > gc.UsernameSize {
		//log.Printf("Supplied username %s is too big\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Check if public key is usable
	_, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		//log.Printf("Incorrect public key from %s when registering: %s\n", u.name, err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Register user into the database
	e := insertUser(h.db, uname, cmd.Args[1])
	if e != nil {
		//log.Printf("User %s alredy exists: %s\n", u.name, err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorExists, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with VERIF or ERR
func loginUser(h *Hub, u User, cmd gc.Command) {
	ran := randText()
	enc, err := gc.EncryptText(ran, u.pubkey)
	if err != nil {
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
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(vpak) // send VERIF

	// Cancel function will be used to stop the following goroutine
	ctx, cancl := context.WithCancel(context.Background())

	// Add to pending verifications
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
			h.verifs.Remove(u.conn)
		case <-ctx.Done():
			// Verification completed by VERIF
			return
		}
	}()
}

// Replies with OK or ERR
func verifyUser(h *Hub, u User, cmd gc.Command) {
	verif, ok := h.verifs.Get(u.conn)

	if !ok {
		//log.Printf("%s is not in verification!\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	if verif.text != string(cmd.Args[1]) || verif.name != u.name {
		// Incorrect verification so we cancel the handshake process
		verif.cancel()
		h.cleanupUser(u.conn)
		//log.Printf("%s verification is incorrect\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorHandshake, u.conn)
		return
	}

	// We modify the tables and cancel the goroutine
	verif.cancel()
	h.users.Add(u.conn, &u)
	h.verifs.Remove(u.conn)

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with OK or ERR
func logoutUser(h *Hub, u User, cmd gc.Command) {
	_, uok := h.users.Get(u.conn)
	_, vok := h.verifs.Get(u.conn)

	if !uok && !vok {
		// If user is in none of the caches we error
		//log.Printf("%s trying to disconnect when not connected\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNoSession, u.conn)
		return
	}

	// Otherwise we cleanup
	h.cleanupUser(u.conn)

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with OK or ERR
func deregisterUser(h *Hub, u User, cmd gc.Command) {
	// Cleanup cache information in any case
	defer h.cleanupUser(u.conn)

	// Delete user if message cache is empty
	e := removeUser(h.db, u.name)
	if e == nil {
		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Database error different than foreign key violation
	if e != ErrorDBConstraint {
		log.Printf("Error when deleting user %s: %s\n", u.name, e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	// The user has cached messages so we just NULL the pubkey
	err := removeKey(h.db, u.name)
	if err != nil {
		log.Printf("Failed deregister user %s: %s\n", u.name, err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with REQ or ERR
func requestUser(h *Hub, u User, cmd gc.Command) {
	k, err := queryUserKey(h.db, username(cmd.Args[0]))
	if err != nil {
		//log.Printf("Requested user can not be queried: %s\n", err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}

	p, e := gc.PubkeytoPEM(k)
	if e != nil {
		//! This means the user's database is corrupted info
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		log.Fatalf("%s has inconsistent database publickey: %s!\n", u.name, err)
		return
	}

	// We reply with the username that was requested as well
	arg := []gc.Arg{
		cmd.Args[0],
		gc.Arg(p),
	}
	pak, e := gc.NewPacket(gc.REQ, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating REQ packet: %s\n", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send REQ
}

// Replies with USRS or ERR
func listUsers(h *Hub, u User, cmd gc.Command) {
	var usrs string

	// Show online users or all
	online := cmd.HD.Info

	// 0x01 is show online
	if online == 0x01 {
		usrs = h.userlist(true)
	} else if online == 0x00 {
		usrs = h.userlist(false)
	} else {
		// Error due to invalid argument in header info
		//log.Printf("Invalid user list argument from %s\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorHeader, u.conn)
		return
	}

	if usrs == "" {
		// Could not find any users matching
		//log.Printf("No users exist when querying userlist\n")
		sendErrorPacket(cmd.HD.ID, gc.ErrorEmpty, u.conn)
		return
	}

	arg := []gc.Arg{
		gc.Arg(usrs),
	}
	pak, e := gc.NewPacket(gc.USRS, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating USRS packet: %s\n", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send USRS
}

// Replies with OK or ERR
// Sends a RECIV if destination user is online
func messageUser(h *Hub, u User, cmd gc.Command) {
	// Check if its online cached
	send, ok := h.findUser(username(cmd.Args[0]))
	if ok {
		// We send the message directly to the connection
		arg := []gc.Arg{
			gc.Arg(u.name),
			cmd.Args[1],
			cmd.Args[2],
		}
		pak, e := gc.NewPacket(gc.RECIV, gc.NullID, gc.EmptyInfo, arg)
		if e != nil {
			log.Printf("Error when creating RECIV packet: %s\n", e)
			sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
			return
		}
		send.conn.Write(pak) // send RECIV (to destination)

		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Otherwise we just send it to the message cache
	uname := username(cmd.Args[0])
	err := cacheMessage(h.db, u.name, uname, cmd.Args[2])
	if err != nil {
		// Error when inserting the message into the cache
		log.Printf("Error when caching a message from %s\n", u.name)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with RECIV or ERR
func recivMessages(h *Hub, u User, cmd gc.Command) {
	// Get the amount of messages needed for allocation
	size, err := queryMessageQuantity(h.db, u.name)
	if err != nil {
		log.Printf("Could not query message quantity for %s: %s\n", u.name, err)
	}
	if size == 0 {
		// Nothing to do
		sendErrorPacket(cmd.HD.ID, gc.ErrorEmpty, u.conn)
		return
	}

	catch, err := queryMessages(h.db, u.name, size)
	if err != nil {
		log.Printf("Could not query messages for %s: %s\n", u.name, err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	chk := catchUp(u.conn, catch, cmd.HD.ID) // send RECIV(s)
	if chk != nil {
		// We do not delete messages in this case
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}

	// Get the timestamp of the newest message as threshold for deletion
	ts := (*catch)[size].stamp
	e := removeMessages(h.db, u.name, ts)
	if e != nil {
		log.Printf("Error when deleting cached messages from %s: %s", u.name, err)
	}
}
