package main

import (
	"log"

	gc "github.com/Sprinter05/gochat/gcspec"
)

/*
[-] ALL   -> Logs every single packet
[I] INFO  -> Logs information about the connection
[E] ERROR -> Log server related errors
[X] FATAL -> Logs when it crashes the program
*/
type Logging int

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
		"[E] Problem in %s due to %s",
		msg,
		err,
	)
}

// Requires ERROR or higher
// Problem running a SQL statement
func (l Logging) DB(data string, err error) {
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
		"[E] Creation of packet %s due to %s\n",
		gc.CodeToString(op),
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
