package hubs

import (
	"errors"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
)

/* LOOKUP */

var adminPerms map[spec.Admin]db.Permission = map[spec.Admin]db.Permission{
	spec.AdminShutdown:    db.ADMIN,
	spec.AdminBroadcast:   db.ADMIN,
	spec.AdminDeregister:  db.ADMIN,
	spec.AdminChangePerms: db.OWNER,
	spec.AdminDisconnect:  db.ADMIN,
	spec.AdminMotd:        db.OWNER,
}

var adminLookup map[spec.Admin]action = map[spec.Admin]action{
	spec.AdminShutdown:    adminShutdown,
	spec.AdminBroadcast:   adminBroadcast,
	spec.AdminDeregister:  adminDeregister,
	spec.AdminChangePerms: adminChangePerms,
	spec.AdminDisconnect:  adminDisconnect,
	spec.AdminMotd:        adminChangeMotd,
}

/* WRAPPER FUNCTIONS */

// Runs an admin operation according to the information
// header field and the arguments provided. All
// admin commands will return either ERR or OK.
func adminOperation(h *Hub, u User, cmd spec.Command) {
	if u.perms == db.USER {
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	op := spec.Admin(cmd.HD.Info)
	fun, ok := adminLookup[op]
	if !ok {
		// Invalid action is trying to be ran
		log.Invalid(spec.AdminString(op), string(u.name))
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	// No need to check for -1 since we already checked if it existed
	args := spec.AdminArgs(op)
	if int(cmd.HD.Args) < args {
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	perms := adminPerms[op]
	if u.perms < perms {
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	fun(h, u, cmd)
}

/* COMMANDS */

// Shuts down the server at a certain time.
//
// Requires ADMIN or more.
// Uses 1 argument for the unix stamp
func adminShutdown(h *Hub, u User, cmd spec.Command) {
	stamp, err := spec.BytesToUnixStamp(cmd.Args[0])
	if err != nil {
		// Invalid number given
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	duration := time.Until(stamp)
	if duration < 0 {
		// Invalid duration
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	// Block until the specified time
	go func() {
		time.Sleep(duration)

		// Send shutdown signal to hub
		h.close()
	}()

	pak, err := spec.NewPacket(spec.SHTDWN, spec.NullID, spec.EmptyInfo, cmd.Args[0])
	if err != nil {
		log.Packet(spec.SHTDWN, err)
		SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}

	// Warn all users of the shutdown
	list := h.users.GetAll()
	for _, v := range list {
		v.conn.Write(pak)
	}

	log.Notice("server shutdown on " + stamp.String())
	SendOKPacket(cmd.HD.ID, u.conn)
}

// Broadcasts a message to all online users.
//
// Requires ADMIN or more and a TLS connection
// Requires 1 argument for the message
func adminBroadcast(h *Hub, u User, cmd spec.Command) {
	if !u.secure {
		// Requires TLS
		SendErrorPacket(cmd.HD.ID, spec.ErrorUnsecure, u.conn)
		return
	}

	// We use the hub function to broadcast messages
	h.Broadcast(string(cmd.Args[0]), u)

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Deregisters a user from the database.
//
// Requires ADMIN or more
// Requires 1 argument for the user
func adminDeregister(h *Hub, u User, cmd spec.Command) {
	uname := string(cmd.Args[0])
	dr, err := db.QueryUser(h.db, uname)
	if err != nil {
		if errors.Is(err, db.ErrorNotFound) {
			// Invalid user provided
			SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		} else {
			SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		}
		return
	}

	if uint(u.perms) <= uint(dr.Permission) {
		// Cannot deregister someone with higher permissions than you
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	err = db.RemoveKey(h.db, string(cmd.Args[0]))
	if err != nil {
		// Failed to change the key of the user
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	dc, ok := h.FindUser(uname)
	if ok {
		// We close the connection with the target,
		// also triggering the cleanup function
		dc.conn.Close()
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Changes the permission level of a user
//
// Requires OWNER or more
// Requires 1 argument for the user and 1 for the level of permissions
func adminChangePerms(h *Hub, u User, cmd spec.Command) {
	dest := string(cmd.Args[0])

	if dest == u.name {
		// Cannot change your own permissions
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	target, err := db.QueryUser(h.db, dest)
	if err != nil {
		if errors.Is(err, db.ErrorNotFound) {
			// Invalid user provided
			SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		} else {
			SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		}
		return
	}

	level, err := spec.BytesToPermission(cmd.Args[1])
	if err != nil {
		// Invalid permission provided
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	check := db.PermissionExists(level)
	if !check {
		// Invalid permisison provided
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	if uint(u.perms) <= level {
		// Cannot change perms that are over your permissions
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	if uint(u.perms) <= uint(target.Permission) {
		// Cannot change permissions of someone with more
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	if uint(target.Permission) == level {
		// Cannot change permissions if they are the same
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	// Update in database, we do not check error
	// because it was already queried
	new := db.Permission(level)
	err = db.ChangePermission(h.db, dest, new)
	if err != nil {
		log.DBError(err)
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
	}

	// Update if online
	chg, ok := h.FindUser(string(cmd.Args[0]))
	if ok {
		chg.perms = new
		go h.Notify(
			spec.HookPermsChange, nil,
			[]byte(dest),
			[]byte{byte(level)},
		)
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Disconnects an online user if it's connected.
//
// Requires ADMIN or more
// Requires 1 argument for the user
func adminDisconnect(h *Hub, u User, cmd spec.Command) {
	dc, ok := h.FindUser(string(cmd.Args[0]))
	if !ok {
		SendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		return
	}

	if u.perms <= dc.perms {
		SendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	// This should trigger the cleanup on
	// the goroutine listening to the client
	dc.conn.Close()

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Changes the MOTD of the server
//
// Requires OWNER or more
// Requires 1 argument for the new MOTD
func adminChangeMotd(h *Hub, u User, cmd spec.Command) {
	h.motd = string(cmd.Args[0])
	SendOKPacket(cmd.HD.ID, u.conn)
}
