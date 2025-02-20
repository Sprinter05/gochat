package main

import (
	"log"
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/* LOOKUP */

// Argument mapping table
func getAdminArguments(ad uint8) uint8 {
	args := map[uint8]uint8{
		gc.AdminShutdown:   1,
		gc.AdminBroadcast:  1,
		gc.AdminDeregister: 1,
		gc.AdminPromote:    1,
		gc.AdminDisconnect: 1,
	}

	// Ok has to be checked on lookup first
	v := args[ad]
	return v
}

// Permission mapping table
func getAdminPermission(ad uint8) Permission {
	perms := map[uint8]Permission{
		gc.AdminShutdown:   ADMIN,
		gc.AdminBroadcast:  ADMIN,
		gc.AdminDeregister: ADMIN,
		gc.AdminPromote:    OWNER,
		gc.AdminDisconnect: ADMIN,
	}

	// Ok has to be checked on lookup first
	v := perms[ad]
	return v
}

// Admin function mapping table
// We do not use a variable as a map cannot be const
func lookupAdmin(ad uint8) (action, error) {
	lookup := map[uint8]action{
		gc.AdminShutdown:   adminShutdown,
		gc.AdminBroadcast:  adminBroadcast,
		gc.AdminDeregister: adminDeregister,
		gc.AdminPromote:    adminPromote,
		gc.AdminDisconnect: adminDisconnect,
	}

	v, ok := lookup[ad]
	if !ok {
		return nil, ErrorDoesNotExist
	}

	return v, nil
}

// Every admin operation replies with either ERR or OK
func adminOperation(h *Hub, u User, cmd gc.Command) {
	if u.perms == USER {
		sendErrorPacket(cmd.HD.ID, gc.ErrorPrivileges, u.conn)
		return
	}

	fun, err := lookupAdmin(cmd.HD.Info)
	if err != nil {
		// Invalid action is trying to be ran
		log.Printf("No admin operation asocciated to %d, skipping!\n", cmd.HD.Info)
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	args := getAdminArguments(cmd.HD.Info)
	if cmd.HD.Args != args {
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	perms := getAdminPermission(cmd.HD.Info)
	if u.perms < perms {
		sendErrorPacket(cmd.HD.ID, gc.ErrorPrivileges, u.conn)
		return
	}

	fun(h, u, cmd)
}

/* COMMANDS */

// Requires ADMIN or more
// Uses 1 argument for the unix stamp
func adminShutdown(h *Hub, u User, cmd gc.Command) {
	stamp := gc.NewUnixStamp(cmd.Args[0])
	if stamp == -1 {
		// Invalid number given
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	duration := stamp - time.Now().Unix()
	if duration < 0 {
		// Invalid duration
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	go func() {
		wait := time.Duration(duration) * time.Second
		time.Sleep(wait)

		// Send shutdown signal to server
		h.shtdwn <- true
	}()

	args := []gc.Arg{
		cmd.Args[0],
	}
	pak, e := gc.NewPacket(gc.SHTDWN, gc.NullID, gc.EmptyInfo, args)
	if e != nil {
		log.Printf("Error when creating SHTDWN packet: %s\n", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}

	// Warn all users of the shutdown
	list := h.users.GetAll()
	for _, v := range list {
		v.conn.Write(pak)
	}

	print := time.Unix(stamp, 0)
	log.Printf("Server shutdown scheduled for %v!\n", print)
	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires ADMIN or more
// Requires 1 argument for the message
func adminBroadcast(h *Hub, u User, cmd gc.Command) {
	// Create packet with the message
	arg := []gc.Arg{
		gc.Arg(u.name + " [ADMIN]"),
		gc.UnixStampNow(),
		cmd.Args[0],
	}
	pak, e := gc.NewPacket(gc.RECIV, gc.NullID, gc.EmptyInfo, arg)
	if e != nil {
		log.Printf("Error when creating RECIV packet: %s\n", e)
		sendErrorPacket(cmd.HD.ID, gc.ErrorPacket, u.conn)
		return
	}

	list := h.users.GetAll()
	for _, v := range list {
		// Send to each user
		v.conn.Write(pak)
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires ADMIN or more
// Requires 1 argument for the user
func adminDeregister(h *Hub, u User, cmd gc.Command) {
	err := removeKey(h.db, username(cmd.Args[0]))
	if err != nil {
		// Failed to change the key of the user
		sendErrorPacket(cmd.HD.ID, gc.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires OWNER or more
// Requires 1 argument for the user
func adminPromote(h *Hub, u User, cmd gc.Command) {
	curr, err := queryUserPerms(h.db, username(cmd.Args[0]))
	if err != nil {
		// Invalid user provided
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

	if curr >= ADMIN {
		// Cannot promote more
		sendErrorPacket(cmd.HD.ID, gc.ErrorInvalid, u.conn)
		return
	}

	e := changePermissions(h.db, u.name, ADMIN)
	if e != nil {
		//! This shouldnt happen as the user was already queried before
		sendErrorPacket(cmd.HD.ID, gc.ErrorUndefined, u.conn)
		log.Fatalf("Impossible to promote user %s!", string(cmd.Args[0]))
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires ADMIN or more
// Requires 1 argument for the user
func adminDisconnect(h *Hub, u User, cmd gc.Command) {
	dc, ok := h.findUser(username(cmd.Args[0]))
	if !ok {
		sendErrorPacket(cmd.HD.ID, gc.ErrorNotFound, u.conn)
		return
	}

	// This should stop the client thread
	// And also cleanup caches
	dc.conn.Close()
}
