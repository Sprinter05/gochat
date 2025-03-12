package hubs

import (
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/spec"
	"github.com/Sprinter05/gochat/server/db"
	"github.com/Sprinter05/gochat/server/model"
)

/* LOOKUP */

var adminArgs map[uint8]uint8 = map[uint8]uint8{
	spec.AdminShutdown:   1,
	spec.AdminBroadcast:  1,
	spec.AdminDeregister: 1,
	spec.AdminPromote:    1,
	spec.AdminDisconnect: 1,
}

var adminPerms map[uint8]model.Permission = map[uint8]model.Permission{
	spec.AdminShutdown:   model.ADMIN,
	spec.AdminBroadcast:  model.ADMIN,
	spec.AdminDeregister: model.ADMIN,
	spec.AdminPromote:    model.OWNER,
	spec.AdminDisconnect: model.ADMIN,
}

var adminLookup map[uint8]action = map[uint8]action{
	spec.AdminShutdown:   adminShutdown,
	spec.AdminBroadcast:  adminBroadcast,
	spec.AdminDeregister: adminDeregister,
	spec.AdminPromote:    adminPromote,
	spec.AdminDisconnect: adminDisconnect,
}

/* WRAPPER FUNCTIONS */

// Every admin operation replies with either ERR or OK
func adminOperation(h *Hub, u User, cmd spec.Command) {
	if u.perms == model.USER {
		sendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	fun, ok := adminLookup[cmd.HD.Info]
	if !ok {
		// Invalid action is trying to be ran
		log.Invalid(spec.AdminString(cmd.HD.Info), string(u.name))
		sendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	args := adminArgs[cmd.HD.Info]
	if cmd.HD.Args != args {
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	perms := adminPerms[cmd.HD.Info]
	if u.perms < perms {
		sendErrorPacket(cmd.HD.ID, spec.ErrorPrivileges, u.conn)
		return
	}

	fun(h, u, cmd)
}

/* COMMANDS */

// Requires ADMIN or more
// Uses 1 argument for the unix stamp
func adminShutdown(h *Hub, u User, cmd spec.Command) {
	stamp := spec.BytesToUnixStamp(cmd.Args[0])
	if stamp == nil {
		// Invalid number given
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	duration := time.Until(*stamp)
	if duration < 0 {
		// Invalid duration
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	go func() {
		wait := time.Duration(duration) * time.Second
		time.Sleep(wait)

		// Send shutdown signal to server
		h.shtdwn <- true
	}()

	pak, e := spec.NewPacket(spec.SHTDWN, spec.NullID, spec.EmptyInfo, cmd.Args[0])
	if e != nil {
		log.Packet(spec.SHTDWN, e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
		return
	}

	// Warn all users of the shutdown
	list := h.users.GetAll()
	for _, v := range list {
		v.conn.Write(pak)
	}

	log.Notice("server shutdown on " + stamp.String())
	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires ADMIN or more
// Requires 1 argument for the message
func adminBroadcast(h *Hub, u User, cmd spec.Command) {
	// Create packet with the message
	pak, e := spec.NewPacket(spec.RECIV, spec.NullID, spec.EmptyInfo,
		[]byte(u.name+" [ADMIN]"),
		spec.UnixStampToBytes(time.Now()),
		cmd.Args[0],
	)
	if e != nil {
		log.Packet(spec.RECIV, e)
		sendErrorPacket(cmd.HD.ID, spec.ErrorPacket, u.conn)
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
func adminDeregister(h *Hub, u User, cmd spec.Command) {
	err := db.RemoveKey(h.db, string(cmd.Args[0]))
	if err != nil {
		// Failed to change the key of the user
		sendErrorPacket(cmd.HD.ID, spec.ErrorServer, u.conn)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires OWNER or more
// Requires 1 argument for the user
func adminPromote(h *Hub, u User, cmd spec.Command) {
	target, err := db.QueryUser(h.db, string(cmd.Args[0]))
	if err != nil {
		// Invalid user provided
		sendErrorPacket(cmd.HD.ID, spec.ErrorArguments, u.conn)
		return
	}

	if target.Permission >= model.ADMIN {
		// Cannot promote more
		sendErrorPacket(cmd.HD.ID, spec.ErrorInvalid, u.conn)
		return
	}

	e := db.ChangePermission(h.db, u.name, model.ADMIN)
	if e != nil {
		//! This shouldnt happen as the user was already queried before
		sendErrorPacket(cmd.HD.ID, spec.ErrorUndefined, u.conn)
		log.Fatal("promotion for "+string(u.name), e)
		return
	}

	sendOKPacket(cmd.HD.ID, u.conn)
}

// Requires ADMIN or more
// Requires 1 argument for the user
func adminDisconnect(h *Hub, u User, cmd spec.Command) {
	dc, ok := h.FindUser(string(cmd.Args[0]))
	if !ok {
		sendErrorPacket(cmd.HD.ID, spec.ErrorNotFound, u.conn)
		return
	}

	// This should stop the client thread
	// And also cleanup caches
	dc.conn.Close()
}
