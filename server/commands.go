package main

import (
	"context"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* LOOKUP */

// Function mapping table
// We do not use a variable as a map cannot be const
func lookupCommand(op gc.Action) (action, error) {
	lookup := map[gc.Action]action{
		gc.REG:    registerUser,
		gc.LOGIN:  loginUser,
		gc.VERIF:  verifyUser,
		gc.LOGOUT: logoutUser,
		gc.DEREG:  deregisterUser,
		gc.REQ:    requestUser,
		gc.USRS:   listUsers,
		gc.MSG:    messageUser,
		gc.RECIV:  recivMessages,
		gc.ADMIN:  adminOperation,
	}

	v, ok := lookup[op]
	if !ok {
		return nil, ErrorDoesNotExist
	}

	return v, nil
}

// Check which action to perform
func procRequest(h *Hub, r Request, u *User) {
	id := r.cmd.HD.Op

	fun, err := lookupCommand(id)
	if err != nil {
		// Invalid action is trying to be ran
		gclog.Invalid(gc.CodeToString(id), string(u.name))
		sendErrorPacket(r.cmd.HD.ID, gc.ErrorInvalid, r.cl)
		return
	}

	// Run command
	fun(h, *u, r.cmd)
}

/* COMMANDS */

// Replies with OK or ERR
// Uses a user with only the net.Conn
func registerUser(h *Hub, u User, cmd gc.Command) {
	uname := username(cmd.Args[0])

	if len(uname) > gc.UsernameSize {
		gclog.User(string(uname), "username registration", gc.ErrorMaxSize)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Check if public key is usable
	_, err := gc.PEMToPubkey(cmd.Args[1])
	if err != nil {
		gclog.User(string(uname), "pubkey registration", ErrorInvalidValue)
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	// Register user into the database
	e := insertUser(h.db, uname, cmd.Args[1])
	if e != nil {
		gclog.User(string(uname), "registration", gc.ErrorExists)
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
		gclog.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We create and send the packet with the enconded text
	arg := []gc.Arg{
		gc.Arg(enc),
	}
	vpak, e := gc.NewPacket(gc.VERIF, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		gclog.Packet(gc.VERIF, e)
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
			gclog.Timeout(string(u.name), "verification")
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
		gclog.User(string(u.name), "verification", ErrorDoesNotExist)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	if verif.text != string(cmd.Args[1]) || verif.name != u.name {
		// Incorrect verification so we cancel the handshake process
		verif.cancel()
		h.cleanupUser(u.conn)
		gclog.User(string(u.name), "verification", ErrorInvalidValue)
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
		gclog.User(string(u.name), "logout", gc.ErrorNoSession)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNoSession, u.conn)
		return
	}

	// Otherwise we cleanup
	h.cleanupUser(u.conn)

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with OK or ERR
func deregisterUser(h *Hub, u User, cmd gc.Command) {
	// Delete user if message cache is empty
	e := removeUser(h.db, u.name)
	if e == nil {
		h.cleanupUser(u.conn)
		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Database error different than foreign key violation
	if e != ErrorDBConstraint {
		gclog.DBQuery(string(u.name)+" deletion", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	// The user has cached messages so we just NULL the pubkey
	err := removeKey(h.db, u.name)
	if err != nil {
		gclog.DBQuery(string(u.name)+" pubkey to null", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	h.cleanupUser(u.conn)
	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with REQ or ERR
func requestUser(h *Hub, u User, cmd gc.Command) {
	req, err := queryUser(h.db, username(cmd.Args[0]))
	if err != nil {
		gclog.DBQuery(string(u.name)+" pubkey", err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}

	p, e := gc.PubkeytoPEM(req.pubkey)
	if e != nil {
		//! This means the user's database is corrupted info
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		gclog.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We reply with the username that was requested as well
	arg := []gc.Arg{
		gc.Arg(req.name),
		gc.Arg(p),
	}
	pak, e := gc.NewPacket(gc.REQ, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		gclog.Packet(gc.REQ, e)
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
		gclog.User(string(u.name), "list argument", ErrorInvalidValue)
		sendErrorPacket(cmd.HD.ID, gc.ErrorHeader, u.conn)
		return
	}

	if usrs == "" {
		// Could not find any users matching
		sendErrorPacket(cmd.HD.ID, gc.ErrorEmpty, u.conn)
		return
	}

	arg := []gc.Arg{
		gc.Arg(usrs),
	}
	pak, e := gc.NewPacket(gc.USRS, cmd.HD.ID, gc.EmptyInfo, arg)
	if e != nil {
		gclog.Packet(gc.USRS, e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send USRS
}

// Replies with OK or ERR
// Sends a RECIV if destination user is online
func messageUser(h *Hub, u User, cmd gc.Command) {
	// Cannot send to self
	if username(cmd.Args[0]) == u.name {
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

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
			gclog.Packet(gc.RECIV, e)
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
		gclog.DBQuery("message caching from "+string(u.name), err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with RECIV or ERR
func recivMessages(h *Hub, u User, cmd gc.Command) {
	msgs, err := queryMessages(h.db, u.name)
	if err != nil {
		// No messages to query
		if err == gc.ErrorEmpty {
			sendErrorPacket(cmd.HD.ID, gc.ErrorEmpty, u.conn)
			return
		}

		// Internal database error
		gclog.DBQuery("messages for"+string(u.name), err)
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	chk := catchUp(u.conn, msgs, cmd.HD.ID) // send RECIV(s)
	if chk != nil {
		// We do not delete messages in this case
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}

	// Get the timestamp of the newest message as threshold for deletion
	size := len(*msgs)
	ts := (*msgs)[size].stamp
	e := removeMessages(h.db, u.name, ts)
	if e != nil {
		// We dont send an ERR here or we would be sending 2 packets
		gclog.DBQuery("deleting cached messages for"+string(u.name), e)
	}

	// Let the client know there are no more catch up RECIVs
	sendOKPacket(cmd.HD.ID, u.conn)
}
