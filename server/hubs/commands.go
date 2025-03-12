package hubs

import (
	"bytes"
	"context"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/model"
)

/* LOOKUP */

// Function mapping table
var cmdLookup map[spec.Action]action = map[spec.Action]action{
	spec.REG:    registerUser,
	spec.LOGIN:  loginUser,
	spec.VERIF:  verifyUser,
	spec.LOGOUT: logoutUser,
	spec.DEREG:  deregisterUser,
	spec.REQ:    requestUser,
	spec.USRS:   listUsers,
	spec.MSG:    messageUser,
	spec.RECIV:  recivMessages,
	spec.ADMIN:  adminOperation,
}

/* WRAPPER FUNCTIONS */

// Check which action to perform and run it
func Process(h *Hub, r Request, u *User) {
	id := r.Command.HD.Op

	fun, ok := cmdLookup[r.Command.HD.Op]
	if !ok {
		// Invalid action is trying to be ran
		log.Invalid(spec.CodeToString(id), string(u.name))
		sendErrorPacket(r.Command.HD.ID, spec.ErrorInvalid, r.Conn)
		return
	}

	// Run command
	fun(h, *u, r.Command)
}

/* COMMANDS */

// Replies with OK or ERR
// Gets a user with only the net.Conn assigned to it
func registerUser(h *Hub, u User, cmd spec.Command) {
	uname := string(cmd.Args[0])

	if len(uname) > spec.UsernameSize {
		log.User(string(uname), "username registration", spec.ErrorMaxSize)
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	// Check if public key is usable
	_, err := spec.PEMToPubkey(cmd.Args[1])
	if err != nil {
		log.User(string(uname), "pubkey registration", model.ErrorInvalidValue)
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	// Register user into the database
	e := db.InsertUser(h.db, uname, cmd.Args[1])
	if e != nil {
		log.User(string(uname), "registration", e)
		if e == model.ErrorDuplicatedKey {
			sendErrorPacket(cmd.HD.ID, spec.ErrorExists, u.conn)
			return
		}
		// Something went wrong
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with VERIF, OK or ERR
func loginUser(h *Hub, u User, cmd spec.Command) {
	// Check if it can be logged in through a reusable token
	if int(cmd.HD.Args) > spec.ServerArgs(cmd.HD.Op) {
		err := h.checkToken(u, cmd.Args[1])
		if err != nil {
			sendErrorPacket(cmd.HD.ID, err, u.conn)
			return
		}

		// Cache the user
		h.users.Add(u.conn, &u)
		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	ran := randText()
	enc, err := spec.EncryptText(ran, u.pubkey)
	if err != nil {
		//! This shouldnt happen, it means the database for the user is corrupted
		sendErrorPacket(cmd.HD.ID, spec.ErrorUndefined, u.conn)
		log.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We create and send the packet with the enconded text
	vpak, e := spec.NewPacket(spec.VERIF, cmd.HD.ID, spec.EmptyInfo, enc)
	if e != nil {
		log.Packet(spec.VERIF, e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(vpak) // send VERIF

	// Cancel function will be used to stop the following goroutine
	ctx, cancl := context.WithCancel(context.Background())

	// Add to pending verifications
	ins := &Verif{
		conn:    u.conn,
		name:    u.name,
		text:    ran,
		cancel:  cancl,
		pending: true,
	}
	h.verifs.Add(u.name, ins)

	// Wait timeout and remove the entry
	// This function is a closure
	go func() {
		w := time.Duration(spec.LoginTimeout) * time.Minute
		select {
		case <-time.After(w):
			log.Timeout(string(u.name), "verification")
			h.verifs.Remove(u.name)
		case <-ctx.Done():
			// Verification completed by VERIF
			return
		}
	}()
}

// Replies with OK or ERR
func verifyUser(h *Hub, u User, cmd spec.Command) {
	verif, ok := h.verifs.Get(u.name)

	if !ok {
		log.User(string(u.name), "verification", model.ErrorDoesNotExist)
		sendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	if !bytes.Equal(verif.text, cmd.Args[1]) || verif.conn != u.conn {
		// Incorrect verification so we cancel the handshake process
		verif.cancel()
		h.Cleanup(u.conn)
		log.User(string(u.name), "verification", model.ErrorInvalidValue)
		sendErrorPacket(cmd.HD.ID, spec.ErrorHandshake, u.conn)
		return
	}

	// We modify the tables and cancel the goroutine
	verif.cancel()
	h.users.Add(u.conn, &u)

	if u.secure {
		// If we are using TLS we mark a soft delete
		verif.pending = false
	} else {
		// Otherwise we remove it
		h.verifs.Remove(u.name)
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with OK or ERR
func logoutUser(h *Hub, u User, cmd spec.Command) {
	_, uok := h.users.Get(u.conn)
	_, vok := h.verifs.Get(u.name)

	if !uok && !vok {
		// If user is in none of the caches we error
		log.User(string(u.name), "logout", spec.ErrorNoSession)
		sendErrorPacket(cmd.HD.ID, spec.ErrorNoSession, u.conn)
		return
	}

	// Otherwise we cleanup
	h.Cleanup(u.conn)

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with OK or ERR
func deregisterUser(h *Hub, u User, cmd spec.Command) {
	// Delete user if message cache is empty
	e := db.RemoveUser(h.db, u.name)
	if e == nil {
		h.Cleanup(u.conn)
		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Database error different than foreign key violation
	if e != model.ErrorDBConstraint {
		log.DB(string(u.name)+"'s deletion", e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	// The user has cached messages so we just NULL the pubkey
	err := db.RemoveKey(h.db, u.name)
	if err != nil {
		log.DB(string(u.name)+"'s pubkey to null", e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	h.Cleanup(u.conn)
	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with REQ or ERR
func requestUser(h *Hub, u User, cmd spec.Command) {
	req, err := h.userFromDB(string(cmd.Args[0]))
	if err != nil {
		log.DB(string(u.name)+"'s pubkey", err)
		sendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		return
	}

	p, e := spec.PubkeytoPEM(req.pubkey)
	if e != nil {
		//! This means the user's database is corrupted info
		sendErrorPacket(cmd.HD.ID, spec.ErrorUndefined, u.conn)
		log.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We reply with the username that was requested as well
	pak, e := spec.NewPacket(spec.REQ, cmd.HD.ID, spec.EmptyInfo,
		[]byte(req.name),
		p,
	)
	if e != nil {
		log.Packet(spec.REQ, e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send REQ
}

// Replies with USRS or ERR
func listUsers(h *Hub, u User, cmd spec.Command) {
	var usrs string

	// Show online users or all
	online := cmd.HD.Info

	// 0x01 is show online
	if online == 0x01 {
		usrs = h.Userlist(true)
	} else if online == 0x00 {
		usrs = h.Userlist(false)
	} else {
		// Error due to invalid argument in header info
		log.User(string(u.name), "list argument", model.ErrorInvalidValue)
		sendErrorPacket(cmd.HD.ID, spec.ErrorHeader, u.conn)
		return
	}

	if usrs == "" {
		// Could not find any users matching
		sendErrorPacket(cmd.HD.ID, spec.ErrorEmpty, u.conn)
		return
	}

	pak, e := spec.NewPacket(spec.USRS, cmd.HD.ID, spec.EmptyInfo, []byte(usrs))
	if e != nil {
		log.Packet(spec.USRS, e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send USRS
}

// Replies with OK or ERR
// Sends a RECIV if destination user is online
// Otherwise stores to the database
func messageUser(h *Hub, u User, cmd spec.Command) {
	// Cannot send to self
	if string(cmd.Args[0]) == u.name {
		sendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	// Check if its online cached
	send, ok := h.FindUser(string(cmd.Args[0]))
	if ok {
		// We send the message directly to the connection
		pak, e := spec.NewPacket(spec.RECIV, spec.NullID, spec.EmptyInfo,
			[]byte(u.name),
			cmd.Args[1],
			cmd.Args[2],
		)
		if e != nil {
			log.Packet(spec.RECIV, e)
			sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
			return
		}
		send.conn.Write(pak) // send RECIV (to destination)

		sendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Otherwise we just send it to the message cache
	uname := string(cmd.Args[0])
	stamp, e := spec.BytesToUnixStamp(cmd.Args[1])
	if e != nil {
		sendErrorPacket(cmd.HD.ID, e, u.conn)
		return
	}
	err := db.CacheMessage(h.db, uname, model.Message{
		Sender:  u.name,
		Content: cmd.Args[2],
		Stamp:   stamp,
	})
	if err != nil {
		if err == model.ErrorDoesNotExist {
			sendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
			return
		}
		// Error when inserting the message into the cache
		log.DB("message caching from "+string(u.name), err)
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Replies with RECIV or ERR
func recivMessages(h *Hub, u User, cmd spec.Command) {
	msgs, err := db.QueryMessages(h.db, u.name)
	if err != nil {
		// No messages to query
		if err == spec.ErrorEmpty {
			sendErrorPacket(cmd.HD.ID, spec.ErrorEmpty, u.conn)
			return
		}

		// Internal database error
		log.DB("messages for "+string(u.name), err)
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	chk := catchUp(u.conn, cmd.HD.ID, msgs...) // send RECIV(s)
	if chk != nil {
		// We do not delete messages in this case
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}

	// Get the timestamp of the newest message as threshold for deletion
	size := len(msgs)
	ts := msgs[size-1].Stamp
	e := db.RemoveMessages(h.db, u.name, ts)
	if e != nil {
		// We dont send an ERR here or we would be sending 2 packets
		log.DB("deleting cached messages for "+string(u.name), e)
	}
}
