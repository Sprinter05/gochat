package hubs

import (
	"bytes"
	"context"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
)

/* TYPES */

// Specifies the functions to run depending on the action code.
type action func(*Hub, User, spec.Command)

// Determines a request coming from a listening connection.
type Request struct {
	Conn    net.Conn     // TCP Connection
	Command spec.Command // Entire command read from the connection
	TLS     bool         // Whether the connection is secure or not
}

// Max amount of requests that can be buffered,
// asocciated channel will block once this limit is reached.
const MaxUserRequests int = 5

/* LOOKUP */

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
	spec.SUB:    subscribeHook,
	spec.UNSUB:  unsubscribeHook,
}

/* WRAPPER FUNCTIONS */

// Check which action is asocciated to an operation
// and runs it, the request needs to have the necessary
// fields for the command to run, and the user should
// be retrieved using the Session() function.
func Process(h *Hub, r Request, u User) {
	id := r.Command.HD.Op

	fun, ok := cmdLookup[r.Command.HD.Op]
	if !ok {
		// Invalid action is trying to be ran
		log.Invalid(spec.CodeToString(id), string(u.name))
		SendErrorPacket(r.Command.HD.ID, spec.ErrorInvalid, r.Conn)
		return
	}

	// Run command
	fun(h, u, r.Command)
}

/* COMMANDS */

// Registers a new user into the database, also filling the
// User struct, but does not log it in.
//
// Replies with OK or ERR
func registerUser(h *Hub, u User, cmd spec.Command) {
	username := string(cmd.Args[0])
	uname := strings.ToLower(username)

	if len(uname) > spec.UsernameSize {
		log.User(string(uname), "username registration", spec.ErrorMaxSize)
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	// Check if the public key is usable
	_, err := spec.PEMToPubkey(cmd.Args[1])
	if err != nil {
		log.User(string(uname), "pubkey registration", spec.ErrorArguments)
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	// Register user into the database
	e := db.InsertUser(h.db, uname, cmd.Args[1])
	if e != nil {
		log.User(string(uname), "registration", e)
		if e == db.ErrorDuplicatedKey {
			SendErrorPacket(cmd.HD.ID, spec.ErrorExists, u.conn)
			return
		}
		// Something went wrong
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Checks if a user exists in the database and sends a
// verification packet to the requesting connection.
//
// Replies with VERIF, OK or ERR
func loginUser(h *Hub, u User, cmd spec.Command) {
	// Check if it can be logged in through a reusable token
	if int(cmd.HD.Args) > spec.ServerArgs(cmd.HD.Op) {
		err := h.checkToken(u, cmd.Args[1])
		if err != nil {
			SendErrorPacket(cmd.HD.ID, err, u.conn)
			return
		}

		// Cache the user
		h.users.Add(u.conn, &u)
		go h.Notify(spec.HookNewLogin)
		SendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	ran := randText()
	enc, err := spec.EncryptText(ran, u.pubkey)
	if err != nil {
		//! This shouldnt happen, it means the database for the user is corrupted
		SendErrorPacket(cmd.HD.ID, spec.ErrorUndefined, u.conn)
		log.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We create and send the packet with the enconded text
	vpak, e := spec.NewPacket(spec.VERIF, cmd.HD.ID, spec.EmptyInfo, enc)
	if e != nil {
		log.Packet(spec.VERIF, e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
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

// Checks if a verification token sent is valid and
// log the user in if so.
//
// Replies with OK or ERR
func verifyUser(h *Hub, u User, cmd spec.Command) {
	verif, ok := h.verifs.Get(u.name)

	if !ok {
		log.User(string(u.name), "verification existance", spec.ErrorNotFound)
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	if !bytes.Equal(verif.text, cmd.Args[1]) || verif.conn != u.conn {
		// Incorrect verification so we cancel the handshake process
		verif.cancel()
		h.Cleanup(u.conn)
		log.User(string(u.name), "verification validation", spec.ErrorHandshake)
		SendErrorPacket(cmd.HD.ID, spec.ErrorHandshake, u.conn)
		return
	}

	// If we get here, it means it was correctly verified
	// We modify the tables and cancel the goroutine
	verif.cancel()
	go h.Notify(spec.HookNewLogin)
	h.users.Add(u.conn, &u)

	if u.secure {
		// If we are using TLS we mark a soft delete,
		// that way it can remain as a reusable token.
		verif.pending = false
	} else {
		// Otherwise we remove it
		h.verifs.Remove(u.name)
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Marks an online user as offline.
//
// Replies with OK or ERR
func logoutUser(h *Hub, u User, cmd spec.Command) {
	_, uok := h.users.Get(u.conn)
	_, vok := h.verifs.Get(u.name)

	if !uok && !vok {
		// If user is in none of the caches we error
		log.User(string(u.name), "logout", spec.ErrorNoSession)
		SendErrorPacket(cmd.HD.ID, spec.ErrorNoSession, u.conn)
		return
	}

	// Otherwise we cleanup
	h.Cleanup(u.conn)

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Removes a user from the database and also logs it out.
//
// Replies with OK or ERR
func deregisterUser(h *Hub, u User, cmd spec.Command) {
	// Delete user if message cache is empty
	e := db.RemoveUser(h.db, u.name)
	if e == nil {
		h.Cleanup(u.conn)
		SendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Database error different than foreign key violation
	if e != db.ErrorForeignKey {
		log.DB(string(u.name)+"'s deletion", e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	// The user has cached messages so we just NULL the pubkey
	err := db.RemoveKey(h.db, u.name)
	if err != nil {
		log.DB(string(u.name)+"'s pubkey to null", e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	// Log the user out
	h.Cleanup(u.conn)
	SendOKPacket(cmd.HD.ID, u.conn)
}

// Requests the public key of another user.
//
// Replies with REQ or ERR
func requestUser(h *Hub, u User, cmd spec.Command) {
	req, err := h.userFromDB(string(cmd.Args[0]))
	if err != nil {
		log.DB(string(u.name)+"'s pubkey", err)
		SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		return
	}

	p, e := spec.PubkeytoPEM(req.pubkey)
	if e != nil {
		//! This means the user's database is corrupted info
		SendErrorPacket(cmd.HD.ID, spec.ErrorUndefined, u.conn)
		log.DBFatal("pubkey", string(u.name), err)
		return
	}

	// We reply with the username that was requested as well
	pak, e := spec.NewPacket(spec.REQ, cmd.HD.ID, spec.EmptyInfo,
		[]byte(req.name),
		p,
		[]byte{
			byte(u.perms),
		},
	)
	if e != nil {
		log.Packet(spec.REQ, e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send REQ
}

// Returns a list (separated with '\n') of all user, either
// only online or all, as specified by the information field.
//
// Replies with USRS or ERR
func listUsers(h *Hub, u User, cmd spec.Command) {
	var usrs string

	// Online/All argument
	online := cmd.HD.Info

	// 0x01 is show online
	if online == 0x01 {
		usrs = h.Userlist(true)
	} else if online == 0x00 {
		usrs = h.Userlist(false)
	} else {
		// Error due to invalid argument in header info
		log.User(string(u.name), "userlist argument", spec.ErrorHeader)
		SendErrorPacket(cmd.HD.ID, spec.ErrorHeader, u.conn)
		return
	}

	if usrs == "" {
		// Could not find any users matching
		SendErrorPacket(cmd.HD.ID, spec.ErrorEmpty, u.conn)
		return
	}

	pak, e := spec.NewPacket(spec.USRS, cmd.HD.ID, spec.EmptyInfo, []byte(usrs))
	if e != nil {
		log.Packet(spec.USRS, e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}
	u.conn.Write(pak) // send USRS
}

// Sends a message to a user, if said user is online, a RECIV
// packet will be sent directly, otherwise it will be stored
// in the database for future retrieval.
//
// Replies with OK or ERR
func messageUser(h *Hub, u User, cmd spec.Command) {
	// Cannot send to self
	if string(cmd.Args[0]) == u.name {
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
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
			SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
			return
		}
		send.conn.Write(pak) // send RECIV (to destination)

		SendOKPacket(cmd.HD.ID, u.conn)
		return
	}

	// Otherwise we just send it to the message cache
	uname := string(cmd.Args[0])
	stamp, e := spec.BytesToUnixStamp(cmd.Args[1])
	if e != nil {
		SendErrorPacket(cmd.HD.ID, e, u.conn)
		return
	}
	err := db.CacheMessage(h.db, uname, spec.Message{
		Sender:  u.name,
		Content: cmd.Args[2],
		Stamp:   stamp,
	})
	if err != nil {
		if err == db.ErrorNotFound {
			SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
			return
		}
		// Error when inserting the message into the cache
		log.DB("message caching from "+string(u.name), err)
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Retrieves all pending messages directed to the user from
// the database. Should be requested right after a log in.
//
// Replies with RECIV or ERR
func recivMessages(h *Hub, u User, cmd spec.Command) {
	msgs, err := db.QueryMessages(h.db, u.name)
	if err != nil {
		// No messages to query
		if err == db.ErrorEmpty {
			SendErrorPacket(cmd.HD.ID, spec.ErrorEmpty, u.conn)
			return
		}

		// Internal database error
		log.DB("messages for "+string(u.name), err)
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	SendOKPacket(cmd.HD.ID, u.conn) // confirm query
	catchUp(u.conn, msgs...)        // send RECIV(s)

	// Get the timestamp of the newest message as threshold for deletion
	size := len(msgs)
	ts := msgs[size-1].Stamp
	e := db.RemoveMessages(h.db, u.name, ts)
	if e != nil {
		// We dont send an ERR here or we would be sending 2 packets
		log.DB("deleting cached messages for "+string(u.name), e)
	}
}

// Subscribes a user to an event to get notified
// whenever said event is triggered.
//
// Replies with OK or ERR
func subscribeHook(h *Hub, u User, cmd spec.Command) {
	hook := spec.Hook(cmd.HD.Info)
	if !slices.Contains(spec.Hooks, hook) && hook != spec.HookAllHooks {
		// Provided hook does not exist
		log.User(string(u.name), "invalid hook", spec.ErrorHeader)
		SendErrorPacket(cmd.HD.ID, spec.ErrorHeader, u.conn)
		return
	}

	// Hooks to be subscribed to
	list := make([]spec.Hook, 0)
	if hook == spec.HookAllHooks {
		list = spec.Hooks
	} else {
		list = append(list, hook)
	}

	for _, v := range list {
		sl, ok := h.subs.Get(v)
		if !ok {
			//! This means the hook slice no longer exists even though it should
			SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
			log.Fatal("hub hook slices", spec.ErrorNotFound)
			return
		}

		if sl.Has(u.conn) {
			if hook == spec.HookAllHooks {
				// If subscribing to everything we just skip
				continue
			}

			// User is already subscribed
			SendErrorPacket(cmd.HD.ID, spec.ErrorExists, u.conn)
			return
		}

		// Otherwise we subscribe the user
		sl.Add(u.conn)
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Unubscribes a user from a hook that they are
// subscribed for.
//
// Replies with OK or ERR
func unsubscribeHook(h *Hub, u User, cmd spec.Command) {
	hook := spec.Hook(cmd.HD.Info)
	if !slices.Contains(spec.Hooks, hook) && hook != spec.HookAllHooks {
		// Provided hook does not exist
		log.User(string(u.name), "invalid hook", spec.ErrorHeader)
		SendErrorPacket(cmd.HD.ID, spec.ErrorHeader, u.conn)
		return
	}

	// Hooks to be unsubscribed from
	list := make([]spec.Hook, 0)
	if hook == spec.HookAllHooks {
		list = spec.Hooks
	} else {
		list = append(list, hook)
	}

	for _, v := range list {
		sl, ok := h.subs.Get(v)
		if !ok {
			//! This means the hook slice no longer exists even though it should
			SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
			log.Fatal("hub hook slices", spec.ErrorNotFound)
			return
		}

		if !sl.Has(u.conn) {
			if hook == spec.HookAllHooks {
				// If unsubscribing to everything we just skip
				continue
			}

			// User cannot be unsubscribed if not subscribed
			SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
			return
		}

		// Otherwise we unsubscribe the user
		sl.Remove(u.conn)
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}
