package hubs

import (
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
)

/* LOOKUP */

var adminArgs map[spec.Admin]uint8 = map[spec.Admin]uint8{
	spec.AdminShutdown:    1,
	spec.AdminBroadcast:   1,
	spec.AdminDeregister:  1,
	spec.AdminChangePerms: 2,
	spec.AdminDisconnect:  1,
}

var adminPerms map[spec.Admin]db.Permission = map[spec.Admin]db.Permission{
	spec.AdminShutdown:    db.ADMIN,
	spec.AdminBroadcast:   db.ADMIN,
	spec.AdminDeregister:  db.ADMIN,
	spec.AdminChangePerms: db.OWNER,
	spec.AdminDisconnect:  db.ADMIN,
}

var adminLookup map[spec.Admin]action = map[spec.Admin]action{
	spec.AdminShutdown:    adminShutdown,
	spec.AdminBroadcast:   adminBroadcast,
	spec.AdminDeregister:  adminDeregister,
	spec.AdminChangePerms: adminChangePerms,
	spec.AdminDisconnect:  adminDisconnect,
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

	args := adminArgs[op]
	if cmd.HD.Args < args {
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
		SendErrorPacket(cmd.HD.ID, err, u.conn)
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
		wait := time.Duration(duration) * time.Second
		time.Sleep(wait)

		// Send shutdown signal to hub
		h.close()
	}()

	pak, e := spec.NewPacket(spec.SHTDWN, spec.NullID, spec.EmptyInfo, cmd.Args[0])
	if e != nil {
		log.Packet(spec.SHTDWN, e)
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
// Requires ADMIN or more
// Requires 1 argument for the message
func adminBroadcast(h *Hub, u User, cmd spec.Command) {
	// Create packet with the message
	pak, e := spec.NewPacket(spec.RECIV, spec.NullID, spec.EmptyInfo,
		[]byte(u.name+" ["+db.PermissionString(u.perms)+"]"),
		spec.UnixStampToBytes(time.Now()),
		cmd.Args[0],
	)
	if e != nil {
		log.Packet(spec.RECIV, e)
		SendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}

	// Goroutine to optimise sending everywhere
	go func() {
		list := h.users.GetAll()
		for _, v := range list {
			// Send to each user
			v.conn.Write(pak)
		}
	}()

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Deregisters a user from the database.
//
// Requires ADMIN or more
// Requires 1 argument for the user
func adminDeregister(h *Hub, u User, cmd spec.Command) {
	err := db.RemoveKey(h.db, string(cmd.Args[0]))
	if err != nil {
		// Failed to change the key of the user
		SendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	SendOKPacket(cmd.HD.ID, u.conn)
}

// Changes the permission level of a user
//
// Requires OWNER or more
// Requires 1 argument for the user and 1 for the level of permissions
func adminChangePerms(h *Hub, u User, cmd spec.Command) {
	target, err := db.QueryUser(h.db, string(cmd.Args[0]))
	if err != nil {
		// Invalid user provided
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	level := db.StringPermission(string(cmd.Args[1]))
	if level == -1 {
		// Invalid permission provided
		SendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	if target.Permission == level {
		// Cannot change permissions if they are the same
		SendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	// Update in database, we do not check error
	// because it was already queried
	db.ChangePermission(h.db, u.name, level)

	// Update if online
	chg, ok := h.FindUser(string(cmd.Args[0]))
	if ok {
		chg.perms = level
		go h.Notify(spec.HookPermsChange, chg.conn)
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

	// This should trigger the cleanup on
	// the goroutine listening to the client
	dc.conn.Close()
}
