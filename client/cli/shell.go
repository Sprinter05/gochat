package cli

// This package includes the core functionality of the gochat client shell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/spec"
)

// Given a string containing a command name, returns its execution function.
func fetchCommand(op string, cmd commands.Command) ShellCommand {
	v, ok := shCommands[strings.ToUpper(op)]
	if !ok {
		cmd.Output(
			fmt.Sprintf("%s: command not found", op),
			commands.ERROR,
		)

		return ShellCommand{}
	}
	return v
}

// Creates a new shell and an option connection and server
func New(static commands.StaticData, conn net.Conn, server db.Server) commands.Command {
	data := commands.NewEmptyData()
	cmds := commands.Command{
		Data:   &data,
		Static: &static,
		Output: Print,
	}

	// Assign data variables
	data.Conn = conn
	data.Server = &server

	if static.Verbose {
		fmt.Println("\033[36mgochat\033[0m shell - type HELP [command] for help")
	}

	commands.WaitConnect(cmds, conn, server)
	if static.Verbose {
		cmds.Output("listening for incoming packets...", commands.INFO)
	}
	go commands.ListenPackets(cmds, func() {})

	go RECIVHandler(cmds)
	go HOOKHandler(cmds)

	return cmds
}

// Starts a shell that allows the client to send packets
// to the gochat server, along with other functionalities.
func Run(data commands.Command) {
	rd := bufio.NewReader(os.Stdin)
	for {
		PrintPrompt(data.Data)
		// Reads user input
		input, readErr := rd.ReadBytes('\n')
		if readErr != nil {
			fmt.Printf("input error: %s\n", readErr)
			continue
		}
		// Trims the input, removing trailing spaces and line jumps
		input = bytes.TrimSpace(input)
		if len(input) == 0 {
			// Empty command, asks for input again
			continue
		}

		op := string(bytes.Fields(input)[0])
		if strings.ToUpper(op) == "EXIT" {
			return
		}

		// Sets up command data
		var args [][]byte
		args = append(args, bytes.Fields(input)[1:]...)

		if strings.ToUpper(op) == "HELP" {
			help(data, args...)
			continue
		}

		// Gets the appropiate command and executes it
		shCmd := fetchCommand(op, data)
		if shCmd.Run == nil {
			continue
		}

		//* Can be changed with context.WithTimeout
		err := shCmd.Run(context.Background(), data, args...)
		if err != nil {
			fmt.Printf("[ERROR] %s: %s\n", op, err)
		}
	}
}

func PrintPrompt(data *commands.Data) {
	connected := ""
	username := ""
	if data.IsLoggedIn() {
		username = data.LocalUser.User.Username
	}

	if !data.IsConnected() {
		connected = "(not connected) "
	}
	fmt.Printf("\033[36m%sgochat(%s) > \033[0m", connected, username)
}

// Shell-specific output function that handles different
// input types accordingly.
func Print(text string, outputType commands.OutputType) {
	prefix := ""
	jump := "\n"
	switch outputType {
	case commands.INTERMEDIATE:
		prefix = "[...] "
	case commands.PACKET:
		prefix = "[PACKET]\n"
	case commands.PROMPT:
		jump = ""
	case commands.ERROR:
		prefix = "[ERROR] "
	case commands.INFO:
		prefix = "[INFO] "
	case commands.RESULT:
		prefix = "[OK] "
	}

	fmt.Printf("%s%s%s", prefix, text, jump)
}

// Shell-specific RECIV handler. Listens
// constantly for incoming RECIV packets
// and performs the necessary shell
// operations.
func RECIVHandler(cmd commands.Command) {
	for {
		reciv, _ := cmd.Data.Waitlist.Get(
			context.Background(),
			commands.Find(0, spec.RECIV),
		)
		decrypted, storeErr := commands.StoreMessage(
			context.Background(), reciv, cmd,
		)
		if storeErr != nil {
			// Removes prompt line
			fmt.Print("\r\033[K")
			fmt.Println(storeErr)
			continue
		}
		printMessage(reciv, decrypted.Content, cmd)
	}
}

// Shell-specific HOOL handler. Listens
// constantly for incoming HOOK packets
// and performs the necessary shell
// operations.
func HOOKHandler(cmd commands.Command) {
	for {
		hook, _ := cmd.Data.Waitlist.Get(
			context.Background(),
			commands.Find(0, spec.HOOK),
		)
		printHook(hook, cmd)
	}
}

// Prints a received message in the shell
func printMessage(reciv spec.Command, decryptedText string, cmd commands.Command) {
	stamp, _ := spec.BytesToUnixStamp(reciv.Args[1])
	// Removes prompt line and rings bell
	fmt.Print("\r\033[K\a")
	fmt.Printf("\033[36m[%s] \033[32m%s\033[0m: %s\n", stamp.String(), reciv.Args[0], decryptedText)
	PrintPrompt(cmd.Data)
}

// Prints a received hook in the shell
func printHook(hook spec.Command, cmd commands.Command) {
	// Removes prompt line and rings bell
	fmt.Print("\r\033[K\a")
	fmt.Printf("\033[0;35m[HOOK] \033[32mHook received\033[0m: Code %d (%s)\n", hook.HD.Info, spec.HookString(spec.Hook(hook.HD.Info)))
	PrintPrompt(cmd.Data)
}
