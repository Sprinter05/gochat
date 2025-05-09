// Implements a global log to be used by either a client or a server process.
// Includes several log levels and functions that handle from
// database problems to internal errors, including any relevant information.
package log

import (
	"fmt"
	"log"
	"net"

	"github.com/Sprinter05/gochat/internal/spec"
)

// Indicates a log level, only the provided global
// variable can be used, as it does not
// support changing the output to something
// that is not standard output or using another variable.
type Logging uint

// Global variable that represents the level.
// This allows use between packages.
// Default level is FATAL.
var Level Logging = FATAL

const (
	FATAL Logging = iota // [X] Logs only when it crashes the program
	ERROR                // [E] Logs relevant server and database errors
	INFO                 // [I] Logs information about the connection and user operations
	ALL                  // [-] Logs every single packet
)

// Logs in any level [*]
//
// Notifies any generic server message.
func Notice(msg string) {
	log.Printf(
		"[*] Notification: %s...\n",
		msg,
	)
}

// Requires FATAL
//
// Informs of a missing environment variable.
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
//
// Generic fatal error.
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
//
// Consistency error on the database.
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
//
// Generic error.
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
//
// Notifies an error on a connection from an IP.
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
//
// Internal database problem.
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
//
// Problem running a SQL statement.
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
//
// Problem when creating packet.
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
//
// Timeout of an operation.
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
//
// Error with data related to a user.
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
//
// Problem when reading from a socket.
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
//
// Invalid operation trying to be performed.
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
//
// Prints a new connection.
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
//
// Prints packet information.
func Request(ip string, cmd spec.Command) {
	if Level < ALL {
		return
	}
	log.Printf(
		"[-] New packet from %s:\n",
		ip,
	)
	cmd.Print(LogPrint)
}

func LogPrint(text string) {
	fmt.Print(text)
}
