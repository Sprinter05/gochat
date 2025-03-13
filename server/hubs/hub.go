package hubs

import (
	"context"
	"net"
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
	db     *gorm.DB                      // Database with all relevant information
	shtdwn context.Context               // Used to wait for a shutdown
	close  context.CancelFunc            // Used to trigger a shutdown
	users  models.Table[net.Conn, *User] // Stores all online users
	verifs models.Table[string, *Verif]  // Stores all verifications and/or reusable tokens
}

/* HUB FUNCTIONS */

// Removes all mentions of a user that just disconnected
// from the hub, except the reusable token if the connection
// is secure (condition that is not checked here).
func (hub *Hub) Cleanup(cl net.Conn) {
	// Cleanup on the users table
	hub.users.Remove(cl)

	// Cleanup on the verification table
	list := hub.verifs.GetAll()
	for _, v := range list {
		if v.conn == cl {
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
	}
}

// Checks if a session is present in the hub (including the database)
// for a connection, and returns the corresponding user if so. If no user
// exists and it is applicable, a newly created user will be returned,
// which should be filled by the function processing the operation.
func (hub *Hub) Session(r Request) (*User, error) {
	op := r.Command.HD.Op

	// Can NOT BE REG LOGIN or VERIF if checking in the cache
	if op != spec.REG && op != spec.LOGIN && op != spec.VERIF {
		cached, err := hub.cachedLogin(r)
		if err == nil {
			// Valid user found in cache, serve request
			return cached, nil
		} else if err != spec.ErrorNotFound {
			// We do not search in the DB if its a different error
			return nil, err
		}
	}

	// Can ONLY be LOGIN or VERIF if checking in the database
	if op == spec.LOGIN || op == spec.VERIF {
		user, e := hub.dbLogin(r)
		if e == nil {
			// User found in database so we serve request
			return user, nil
		} else if e != spec.ErrorNotFound {
			// We do not create a new user if its a different error
			return nil, e
		}
	}

	// Can only be REG if no user was found
	// So if its not REG we error
	if op != spec.REG {
		if op == spec.LOGIN {
			// User did not exist when trying to search previously
			sendErrorPacket(r.Command.HD.ID, spec.ErrorNotFound, r.Conn)
		} else {
			// Cannot do anything without an account
			sendErrorPacket(r.Command.HD.ID, spec.ErrorNoSession, r.Conn)
		}
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
			sendErrorPacket(r.Command.HD.ID, spec.ErrorLogin, r.Conn)
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
		// User is cached and the session can be returned
		return v, nil
	}

	if id == spec.LOGIN {
		// We check if the user is logged in from another IP
		_, ipok := hub.FindUser(string(r.Command.Args[0]))
		if ipok {
			// Cannot have two sessions of the same user
			sendErrorPacket(r.Command.HD.ID, spec.ErrorDupSession, r.Conn)
			return nil, spec.ErrorDupSession

		}
	}

	// Otherwise the user is not found
	return nil, spec.ErrorNotFound
}

/* HUB MAIN */

// Initialises all data structures the hub needs to function:
// database, shutdown context and table sizes.
func NewHub(database *gorm.DB, ctx context.Context, cancel context.CancelFunc, size int) *Hub {
	hub := &Hub{
		shtdwn: ctx,
		close:  cancel,
		users:  models.NewTable[net.Conn, *User](size),
		verifs: models.NewTable[string, *Verif](size),
		db:     database,
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
