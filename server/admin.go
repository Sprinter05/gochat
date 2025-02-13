package main

import (
	"time"

	gc "github.com/Sprinter05/gochat/gcspec"
)

// Every admin operation replies with either ERR or OK
func adminOperation(h *Hub, u *User, cmd gc.Command) {
	if u.perms == USER {
		sendErrorPacket(cmd.HD.ID, gc.ErrorPrivileges, u.conn)
		return
	}
}

// Requires ADMIN or more
// Uses 1 argument for the unix stamp
func scheduleShutdown(h *Hub, u *User, cmd gc.Command) {
	if u.perms < ADMIN {
		sendErrorPacket(cmd.HD.ID, gc.ErrorPrivileges, u.conn)
		return
	}

	if cmd.HD.Args != 1 {
		sendErrorPacket(cmd.HD.ID, gc.ErrorArguments, u.conn)
		return
	}

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
		close(h.req)
	}()

	sendOKPacket(cmd.HD.ID, u.conn)
}
