// This package implements all functionality for working with
// a concurrent access point for all common funcionality between
// clients.
package hubs

import (
	"context"
	"net"
	"slices"
	"time"

	"github.com/Sprinter05/gochat/internal/log"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/Sprinter05/gochat/internal/spec"
	"gorm.io/gorm"
)

/* TYPES */

// Main data structure that stores all information shared
// by all client connections. It is safe to use concurrently.
type Hub struct {
	db     *gorm.DB                                         // Database with all relevant information
	motd   string                                           // Initial message sent to all clients
	close  context.CancelFunc                               // Used to trigger a shutdown
	users  models.Table[net.Conn, *User]                    // Stores all online users
	verifs models.Table[string, *Verif]                     // Stores all verifications and/or reusable tokens
	subs   models.Table[spec.Hook, *models.Slice[net.Conn]] // Stores all users subscribed to an event
}

/* HUB FUNCTIONS */

// Returns the message of the day that is
// currently active
func (hub *Hub) Motd() string {
	return hub.motd
}

// Notifies of a hook to all relevant connections. An
// optional "only" list of connections can be provided
// to only notify those specified in said list. It is
// safe to run this function concurrently.
func (hub *Hub) Notify(h spec.Hook, only ...net.Conn) {
	sl, ok := hub.subs.Get(h)
	if !ok {
		//! This means the hook slice no longer exists even though it should
		log.Fatal("hub hook slices", spec.ErrorNotFound)
		return
	}

	pak, err := spec.NewPacket(spec.HOOK, spec.NullID, byte(h))
	if err != nil {
		log.Packet(spec.HOOK, err)
		return
	}

	list := sl.Copy(0)
	if list == nil {
		// No connection to notify
		return
	}

	for _, v := range list {
		if only != nil && !slices.Contains(only, v) {
			// The connection is not of the only ones to notify
			continue
		}
		// Otherwise we notify
		v.Write(pak)
	}

}

// Removes all mentions of a user that just disconnected
// from the hub, except the reusable token if the connection
// is secure (condition that is not checked here).
func (hub *Hub) Cleanup(cl net.Conn) {
	// Cleanup on the users table
	hub.users.Remove(cl)
	go hub.Notify(spec.HookNewLogout)

	// Cleanup on the verification table
	list := hub.verifs.GetAll()
	for _, v := range list {
		if v.conn != cl {
			// Not the one we are looking for
			continue
		}

		hub.verifs.Remove(v.name)

		// If not pending we assume the connection was secure
		if !v.pending {
			// We assign a nil connection to prevent any possible problems
			v.conn = nil
			v.expiry = time.Now().Add(
				time.Duration(spec.TokenExpiration) * time.Minute,
			)
			hub.verifs.Add(v.name, v)
		}
	}

	// Cleanup on the hooks table
	removeFromHooks(hub, cl)
}

// Checks if a session is present in the hub (including the database)
// for a connection, and returns the corresponding user if so. If no user
// exists and it is applicable, a newly created user will be returned,
// which should be filled by the function processing the operation.
//
// Returns a specification error
func (hub *Hub) Session(r Request) (*User, error) {
	op := r.Command.HD.Op

	// Check for users online in any situation
	cached, err := hub.cachedLogin(r)
	if err == nil {
		// Valid user found in cache, serve request
		return cached, nil
	} else if err != spec.ErrorNotFound {
		// We do not continue checking if its a different error
		return nil, err
	}

	// Check for users in the database
	// only if we are either in LOGIN or VERIF
	if op == spec.LOGIN || op == spec.VERIF {
		user, err := hub.dbLogin(r)
		if err == nil {
			// User found in database so we serve request
			return user, nil
		} else {
			// We do not create a new user if there was an error
			return nil, err
		}
	}

	// Can only be REG if no user was found
	// So if its not REG we error
	if op != spec.REG {
		// Cannot do anything without an account
		return nil, spec.ErrorNoSession
	}

	// Newly created user
	// The REG function is expected to fill the rest of the struct
	return &User{
		conn: r.Conn,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Checks if there is a user in the database, returning an
// specification error if not.
func (hub *Hub) dbLogin(r Request) (*User, error) {
	u := string(r.Command.Args[0])
	user, err := hub.userFromDB(u)
	if err != nil {
		if err == spec.ErrorCorrupted || err == spec.ErrorServer {
			return nil, spec.ErrorLogin
		}
		return nil, err
	}

	// Assign connection and if said connection is secure
	user.conn = r.Conn
	user.secure = r.TLS
	return user, nil
}

// Check if the user is already online, returning an
// specification error if not.
func (hub *Hub) cachedLogin(r Request) (*User, error) {
	id := r.Command.HD.Op

	v, ok := hub.users.Get(r.Conn)
	if ok {
		// Cannot perform these operations if already online
		if id == spec.REG || id == spec.LOGIN || id == spec.VERIF {
			return nil, spec.ErrorInvalid
		}

		// User is cached and the session can be returned
		return v, nil
	}

	if id == spec.LOGIN {
		// We check if the user is logged in from another IP
		dup, ipok := hub.FindUser(string(r.Command.Args[0]))
		if ipok {
			// Cannot have two sessions of the same user
			go hub.Notify(spec.HookDuplicateSession, dup.conn)
			return nil, spec.ErrorDupSession
		}
	}

	// Otherwise the user is not found
	return nil, spec.ErrorNotFound
}

/* HUB MAIN */

// Initialises all data structures the hub needs to function:
// database, shutdown context and table sizes.
func NewHub(database *gorm.DB, cancel context.CancelFunc, size uint, motd string) *Hub {
	// Allocate fields
	hub := &Hub{
		close:  cancel,
		users:  models.NewTable[net.Conn, *User](size),
		verifs: models.NewTable[string, *Verif](size),
		subs:   models.NewTable[spec.Hook, *models.Slice[net.Conn]](uint(len(spec.Hooks))),
		db:     database,
		motd:   motd,
	}

	// Allocate subscription lists
	for _, h := range spec.Hooks {
		list := models.NewSlice[net.Conn](size)
		hub.subs.Add(h, &list)
	}

	return hub
}

// Blocking function that waits until a shutdown is triggered,
// cleaning up all necessary resources and sockets, allowing for
// the caling function to safely exit the program.
func (hub *Hub) Wait(ctx context.Context, socks ...net.Listener) {
	// Wait until the shutdown happens
	<-ctx.Done()

	// Disconnect all users
	list := hub.users.GetAll()
	for _, v := range list {
		// This should trigger the cleanup function that is
		// listening to connections
		v.conn.Close()
	}

	// Wait a bit for everything to close
	time.Sleep(time.Second)

	log.Notice("inminent server shutdown")

	// Close sockets
	hub.close()
	for _, v := range socks {
		// This will stop the blocking and make them check the context,
		// triggering them to close
		v.Close()
	}
}
