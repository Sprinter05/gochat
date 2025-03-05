package main

import (
	"log"
	"net"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/*
[-] ALL   -> Logs every single packet
[I] INFO  -> Logs information about the connection
[E] ERROR -> Log server related errors
[X] FATAL -> Logs when it crashes the program
*/
type Logging uint

// FATAL is the lowest, ALL is the highest
const (
	FATAL Logging = iota
	ERROR
	INFO
	ALL
)

// Logs in any case
// Notifies any generic server message
func (l Logging) Notice(msg string) {
	log.Printf(
		"[*] Notifying %s...\n	",
		msg,
	)
}

// Requires FATAL
// Informs of a missing environment variable
func (l Logging) Environ(envvar string) {
	if l < FATAL {
		return
	}
	log.Fatalf(
		"[X] Missing environment variable %s!\n",
		envvar,
	)
}

// Requires FATAL
// Generic fatal error
func (l Logging) Fatal(msg string, err error) {
	if l < FATAL {
		return
	}
	log.Fatalf(
		"[X] Fatal problem in %s due to %s\n",
		msg,
		err,
	)
}

// Requires FATAL
// Consistency error on the database
func (l Logging) DBFatal(data string, user string, err error) {
	if l < FATAL {
		return
	}
	log.Fatalf(
		"[X] Inconsistent %s on database for %s due to %s\n",
		data,
		user,
		err,
	)
}

// Requires ERROR or higher
// Generic error
func (l Logging) Error(msg string, err error) {
	if l < ERROR {
		return
	}
	log.Printf(
		"[E] Problem in %s due to %s\n",
		msg,
		err,
	)
}

// Requires ERROR or higher
// Notifies an error on an IP
func (l Logging) IP(msg string, ip net.Addr) {
	if l < ERROR {
		return
	}
	log.Printf(
		"[E] Problem with connection from %s due to %s\n",
		ip.String(),
		msg,
	)
}

// Requires ERROR or higher
// Internal database problem
func (l Logging) DBError(err error) {
	if l < ERROR {
		return
	}
	log.Printf(
		"[E] Database error: %s\n",
		err,
	)
}

// Requires ERROR or higher
// Problem running a SQL statement
func (l Logging) DBQuery(data string, err error) {
	if l < ERROR {
		return
	}
	log.Printf(
		"[E] Problem requesting %s from database due to %s\n",
		data,
		err,
	)
}

// Requires ERROR or higher
// Problem when creating packet
func (l Logging) Packet(op gc.Action, err error) {
	if l < ERROR {
		return
	}
	log.Printf(
		"[E] Failure in creation of packet %s due to %s\n",
		gc.CodeToString(op),
		err,
	)
}

// Requires INFO or higher
// Timeout due to timer finishing
func (l Logging) Timeout(user string, msg string) {
	if l < INFO {
		return
	}
	log.Printf(
		"[I] Action timeout during %s for %s\n",
		msg,
		user,
	)
}

// Requires INFO or higher
// Error with data
func (l Logging) User(user string, data string, err error) {
	if l < INFO {
		return
	}
	log.Printf(
		"[I] Problem with %s in %s request due to %s\n",
		user,
		data,
		err,
	)
}

// Requires INFO or higher
// Problem when reading from a socket
func (l Logging) Read(subj string, ip string, err error) {
	if l < INFO {
		return
	}
	log.Printf(
		"[I] Error reading %s from address %s due to %s\n",
		subj,
		ip,
		err,
	)
}

// Requires INFO or higher
// Invalid operation
func (l Logging) Invalid(op string, user string) {
	if l < INFO {
		return
	}
	log.Printf(
		"[I] No operation asocciated to %s on request from %s, skipping!\n",
		op,
		user,
	)
}

// Requires ALL
// Prints a new connection
func (l Logging) Connection(ip string, closed bool) {
	if l < ALL {
		return
	}
	if closed {
		log.Printf(
			"[-] Connection from %s closed!",
			ip,
		)
	} else {
		log.Printf(
			"[-] New connection from %s!",
			ip,
		)
	}
}

// Requires ALL
// Prints packet information
func (l Logging) Request(ip string, cmd gc.Command) {
	if l < ALL {
		return
	}
	log.Printf(
		"[-] New packet from %s:\n",
		ip,
	)
	cmd.Print()
}
