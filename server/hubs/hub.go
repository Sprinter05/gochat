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
	shtdwn context.Context                                  // Used to wait for a shutdown
	close  context.CancelFunc                               // Used to trigger a shutdown
	users  models.Table[net.Conn, *User]                    // Stores all online users
	verifs models.Table[string, *Verif]                     // Stores all verifications and/or reusable tokens
	subs   models.Table[spec.Hook, *models.Slice[net.Conn]] // Stores all users subscribed to an event
}

/* HUB FUNCTIONS */

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
		user, e := hub.dbLogin(r)
		if e == nil {
			// User found in database so we serve request
			return user, nil
		} else {
			if e == spec.ErrorNotFound {
				// User did not exist when trying to search previously
				SendErrorPacket(r.Command.HD.ID, spec.ErrorNotFound, r.Conn)
			}

			// We do not create a new user if there was an error
			return nil, e
		}
	}

	// Can only be REG if no user was found
	// So if its not REG we error
	if op != spec.REG {
		// Cannot do anything without an account
		SendErrorPacket(r.Command.HD.ID, spec.ErrorNoSession, r.Conn)
		return nil, spec.ErrorNoSession
	}

	// Newly created user
	// The REG function is expected to fill the rest of the struct
	return &User{
		conn: r.Conn,
	}, nil
}

/* HUB LOGIN FUNCTIONS */

// Checks if there is a user in the database
// Be careful with sending error packets in the calling function
func (hub *Hub) dbLogin(r Request) (*User, error) {
	u := string(r.Command.Args[0])
	user, e := hub.userFromDB(u)
	if e != nil {
		if e != spec.ErrorNotFound {
			SendErrorPacket(r.Command.HD.ID, spec.ErrorLogin, r.Conn)
		}
		return nil, e
	}

	// Assign connection and if said connection is secure
	user.conn = r.Conn
	user.secure = r.TLS
	return user, nil
}

// Check if the user is already online
// Be careful with sending error packets in the calling function
func (hub *Hub) cachedLogin(r Request) (*User, error) {
	id := r.Command.HD.Op

	v, ok := hub.users.Get(r.Conn)
	if ok {
		// Cannot perform these operations if already online
		if id == spec.REG || id == spec.LOGIN || id == spec.VERIF {
			SendErrorPacket(r.Command.HD.ID, spec.ErrorInvalid, r.Conn)
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
			SendErrorPacket(r.Command.HD.ID, spec.ErrorDupSession, r.Conn)
			return nil, spec.ErrorDupSession
		}
	}

	// Otherwise the user is not found
	return nil, spec.ErrorNotFound
}

/* HUB MAIN */

// Initialises all data structures the hub needs to function:
// database, shutdown context and table sizes.
func NewHub(database *gorm.DB, ctx context.Context, cancel context.CancelFunc, size uint) *Hub {
	// Allocate fields
	hub := &Hub{
		shtdwn: ctx,
		close:  cancel,
		users:  models.NewTable[net.Conn, *User](size),
		verifs: models.NewTable[string, *Verif](size),
		subs:   models.NewTable[spec.Hook, *models.Slice[net.Conn]](uint(len(spec.Hooks))),
		db:     database,
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
func (hub *Hub) Wait(socks ...net.Listener) {
	// Wait until the shutdown happens
	<-hub.shtdwn.Done()

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
