package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Sprinter05/gochat/server/db"
	"gorm.io/gorm"
)

/* TYPE DEFINITIONS */

// Specifies a server shell to
// perform remote operations on the
// database.
type Shell struct {
	db  *gorm.DB      // Database connection
	log *os.File      // File where database logs go
	rd  *bufio.Reader // Input reader
	ip  net.Addr      // Remote database address
}

// Function that specifies a shell command
type shellFunc func(*Shell, []string)

/* ERRORS */

var (
	ErrorInvalidCmd error = errors.New("invalid command given")   // invalid command given
	ErrorFewArgs    error = errors.New("too few arguments given") // too few arguments given
)

/* LOOKUP TABLES */

var lookupShell map[string]shellFunc = map[string]shellFunc{
	"SETOWNER":   ownerUser,
	"CLEARCACHE": clearCache,
	"HELP":       shellHelp,
}

var shellArgs map[string]uint = map[string]uint{
	"SETOWNER":   1,
	"CLEARCACHE": 1,
	"HELP":       0,
}

// Returns the function and minimum number of
// arguments required to run a command
func getShellCommand(cmd string) (shellFunc, uint, bool) {
	f, fOk := lookupShell[cmd]
	a, aOk := shellArgs[cmd]

	if !fOk || !aOk {
		return f, a, false
	}

	return f, a, true
}

/* COMMANDS */

// Prints the help message with info about all commands
func shellHelp(shell *Shell, args []string) {
	fmt.Print(
		"SETOWNER <username>: Sets a user as owner of the server\n" +
			"CLEARCACHE <destination>: Clears the message cache of a user\n" +
			"EXIT: Exits the shell\n",
	)
}

// Sets the given user to be owner of the server
// by changing it in the database
func ownerUser(shell *Shell, args []string) {
	err := db.ChangePermission(
		shell.db,
		args[0],
		db.OWNER,
	)

	if err != nil {
		shell.showError(err)
		return
	}

	shell.showWarn("specified user must log in again to confirm changes")
	shell.showOk()
}

// Deletes all messages from the cache targeting
// a specific user
func clearCache(shell *Shell, args []string) {
	err := db.RemoveMessages(
		shell.db,
		args[0],
		time.Now(),
	)

	if err != nil {
		shell.showError(err)
		return
	}

	shell.showOk()
}

/* SHELL FUNCTIONS */

// Loops the shell execution forever by
// reading a command and executing it
func (shell *Shell) Run() {
	sqldb, _ := shell.db.DB()
	defer sqldb.Close()
	defer shell.log.Close()

	fmt.Print("Connected to server database, use HELP for information\n")

	for {
		shell.showPrompt()

		plain, err := shell.rd.ReadBytes('\n')
		if err != nil {
			panic(err)
		}

		text := bytes.TrimSpace(plain)

		input := strings.Split(string(text), " ")
		if len(input) == 0 {
			continue
		}

		if input[0] == "EXIT" {
			return
		}

		fun, args, ok := getShellCommand(input[0])
		if !ok {
			shell.showError(ErrorInvalidCmd)
			continue
		}

		if (len(input) - 1) < int(args) {
			shell.showError(ErrorFewArgs)
			continue
		}

		fun(shell, input[1:])
	}
}

// Shows a confirmation message
func (shell *Shell) showOk() {
	fmt.Print(
		"[-] Operation completed\n",
	)
}

// Shows a warning message with a given text
func (shell *Shell) showWarn(text string) {
	fmt.Printf(
		"[!] Warning: %s\n",
		text,
	)
}

// Shows an error message
func (shell *Shell) showError(err error) {
	fmt.Printf(
		"[X] Problem occurred: %s\n",
		err,
	)
}

// Prints the shell prompt text
func (shell *Shell) showPrompt() {
	fmt.Printf(
		"\033[36mdatabase@%s > \033[0m",
		shell.ip.String(),
	)
}

// Returns a shell struct with all the necessary
// fields and the database connection
func setupShell(config Config) Shell {
	// Setup database logging file
	f := setupDBLog(config)
	dblog := stdlog.New(f, "", stdlog.LstdFlags)

	// Setup database address
	str := fmt.Sprintf(
		"%s:%d",
		*config.Database.Address,
		*config.Database.Port,
	)
	addr, _ := net.ResolveTCPAddr("tcp", str)

	// Connect to database
	database := db.Connect(dblog, config.Database)

	// Read commands from standard input
	rd := bufio.NewReader(os.Stdin)

	return Shell{
		db:  database,
		log: f,
		rd:  rd,
		ip:  addr,
	}
}
