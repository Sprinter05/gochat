package log

import (
	"log"
	"net"

	"github.com/Sprinter05/gochat/internal/spec"
)

/*
[-] ALL   -> Logs every single packet
[I] INFO  -> Logs information about the connection
[E] ERROR -> Log server related errors
[X] FATAL -> Logs when it crashes the program
*/
type Logging uint

// Global variable
var Level Logging = FATAL

// FATAL is the lowest, ALL is the highest
const (
	FATAL Logging = iota
	ERROR
	INFO
	ALL
)

// Logs in any case
// Notifies any generic server message
func Notice(msg string) {
	log.Printf(
		"[*] Notifying %s...\n	",
		msg,
	)
}

// Requires FATAL
// Informs of a missing environment variable
func Environ(envvar string) {
	if Level < FATAL {
		return
	}
	log.Fatalf(
		"[X] Missing environment variable %s!\n",
		envvar,
	)
}

// Requires FATAL
// Generic fatal error
func Fatal(msg string, err error) {
	if Level < FATAL {
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
func DBFatal(data string, user string, err error) {
	if Level < FATAL {
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
func Error(msg string, err error) {
	if Level < ERROR {
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
func IP(msg string, ip net.Addr) {
	if Level < ERROR {
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
func DBError(err error) {
	if Level < ERROR {
		return
	}
	log.Printf(
		"[E] Database error: %s\n",
		err,
	)
}

// Requires ERROR or higher
// Problem running a SQL statement
func DB(data string, err error) {
	if Level < ERROR {
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
func Packet(op spec.Action, err error) {
	if Level < ERROR {
		return
	}
	log.Printf(
		"[E] Failure in creation of packet %s due to %s\n",
		spec.CodeToString(op),
		err,
	)
}

// Requires INFO or higher
// Timeout due to timer finishing
func Timeout(user string, msg string) {
	if Level < INFO {
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
func User(user string, data string, err error) {
	if Level < INFO {
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
func Read(subj string, ip string, err error) {
	if Level < INFO {
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
func Invalid(op string, user string) {
	if Level < INFO {
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
func Connection(ip string, closed bool) {
	if Level < ALL {
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
func Request(ip string, cmd spec.Command) {
	if Level < ALL {
		return
	}
	log.Printf(
		"[-] New packet from %s:\n",
		ip,
	)
	cmd.Print()
}
