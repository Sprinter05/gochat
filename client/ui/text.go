package ui

import (
	"fmt"
	"net"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
)

/* TEXT */

const KeybindHelp string = `
[-::u]Keybinds Manual:[-::-]

[yellow::b]Ctrl-Alt-L/Ctrl-Shift-L[-::-]: Show/Hide help window
	- Keybinds for the [-::b]chat window[-::-] also apply in here

[yellow::b]Ctrl-Q[-::-]: Exit program

[yellow::b]Ctrl-T[-::-]: Focus chat/input window
	- In the [-::b]chat window[-::-] use [green]Up/Down[-::-] to move
	- In the [-::b]chat window[-::-] use [green]ESC[-::-] to scroll down to the end
	- In the [-::b]chat window[-::-] use [green]Shift-ESC/Alt-ESC[-::-] to scroll up to the beggining
	- In the [-::b]input window[-::-] use [green]ESC[-::-] to clear the text
	- In the [-::b]input window[-::-] use [green]Alt-Enter/Shift-Enter[-::-] to add a newline
	- In the [-::b]input window[-::-] use [green]Up[-::-] to browse through the history of commands ran.

[yellow::b]Ctrl-K + Ctrl-N[-::-]: Create a new buffer
	- [green]ESC[-::-] to cancel
	- [green]Enter[-::-] to confirm

[yellow::b]Ctrl-K + Ctrl-W/Ctrl-H[-::-]: Hide currently focused buffer
	- It can be shown again by creating a buffer with the same name
	
[yellow::b]Ctrl-K + Ctrl-X[-::-]: Delete currently focused buffer

[yellow::b]Ctrl-K[-::-] + [green::b]1-z[-::-]: Jump to specific buffer
	- Press [green]ESC[-::-] to cancel the jump

[yellow::b]Ctrl-S + Ctrl-N[-::-]: Create a new server
	- [green]ESC[-::-] to cancel
	- [green]Enter[-::-] to confirm the different steps
	
[yellow::b]Ctrl-S + Ctrl-W/Ctrl-H[-::-]: Hide currently focused server
	- It can be shown again by typing its name when creating a new server

[yellow::b]Ctrl-S + Ctrl-X[-::-]: Delete currently focused server
	- This will permanantely delete all asocciated data to the server except users
	- Users registered in the deleted server will become "dangling" as they are no longer asocciated to a server

[yellow::b]Ctrl-S[-::-] + [green::b]1-9[-::-]: Jump to specific server
	- Press [green]ESC[-::-] to cancel the jump
	
[yellow::b]Ctrl-G[-::-]: Open the Quick Switcher
	- This will allow you to jump to a desired buffer by typing its name
	- It includes an autocomplete that you can fill using [green]Tab[-::-]
	
[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Shift-Up/Down[-::-]: Go to next/previous server

[yellow::b]Ctrl-B[-::-]: Show/Hide buffer list

[yellow::b]Ctrl-U[-::-]: Show/Hide user list

[yellow::b]Ctrl-R[-::-]: Redraw screen
`

const CommandHelp string = `
[-::u]Commands Manual:[-::-]

[yellow::b]/version[-::-]: Displays the current version of the client and protocol

[yellow::b]/servers[-::-]: Displays the list of all servers that are in the database

[yellow::b]/buffers[-::-]: Displays a list of all buffers in the current server
	- Those that have been hidden will also be displayed
	
[yellow::b]/clear[-::-]: Clears all system messages in the current buffer

[yellow::b]/config[-::-]: Shows all current configuration options
	- It will display both the name and value of the option
	- It will only display those available in the current server

[yellow::b]/set[-::-] [green]<option>[-] [green]<value>[-]: Updates a value in the configuration
	- The option name is case sensitive
	- The option name must follow the same format as the configuration shows
	
[yellow::b]/connect[-::-] [blue](-noverify)[-] [blue](-noidle)[-]: Connects to the currently active server using its address
	- This will fail if the server is local
	- If the connection is TLS and "-noverify" is used, certificates will not be checked
	- If "-noidle" is used, the client will try to avoid being disconnected for inactivity

[yellow::b]/register[-::-] [green]<username>[-]: Creates a new account in the currently active server
	- A popup asking for a password to register will show up when creating a new account
	- No two accounts with the same name can exist in one single server
	- You need an active connection to use this command
	
[yellow::b]/deregister[-::-] [green]<username>[-]: Deletes the specified account	
	- A popup asking for the password asocciated to the account will show up
	- This will remove the account both in the remote server and local client

[yellow::b]/import[-::-] [green]<username>[-] [green]<path>[-]: Registers a new user from an existing key
	- The path provided must be related to the directory from which the program was ran
	- The provided private key must be RSA 4096 bits in PEM PKCS1 format
	- A popup asking for a password for the imported account will show up

[yellow::b]/export[-::-] [green]<username>[-]: Exports the private key of an existing local user
	- The specified user must be registered on the server on which the command is ran	
	- A popup asking for the password asocciated to the account will show up
	- The key will be put in a file in the directory from which the program was ran
	- The fill will be called <username>.priv and will be in PEM PKCS1 format (RSA 4096 bits)

[yellow::b]/login[-::-] [green]<username>[-]: Tries to login in the server with an account
	- A popup asking for the password asocciated to the account will show up
	- You need an active connection to use this command

[yellow::b]/logout[-::-]: Logs out of your account in the currently active server
	- You need an active connection to use this command

[yellow::b]/disconnect[-::-]: Interrumps the connection with the currently active server
	- You need an active connection to use this command

[yellow::b]/users[-::-] [green]<remote/local>[-] [green]<all/online/server>[-] [blue](-perms)[-]: Shows a list of users according to the specified filter
	- [cyan]"remote all"[-] will display all users registered on the remote server (requires connection)
	- [cyan]"remote online"[-] will display all connected accounts in the server (requires connection)
	- [cyan]"local all"[-] will display accounts created for for all servers on this client
	- [cyan]"local server"[-] will display all local accounts for that server
	- For the [cyan]"remote"[-] options you can optionally pass "-perms" to show permission levels
	
[yellow::b]/subscribe[-::-] [green]<hook>[-]: Subscribes to a specific event in the server
	- [cyan]"new_login"[-] will update the userlist whenever a new user logs in
	- [cyan]"new_logout"[-] will update the userlist whenever a user logs out
	- [cyan]"duplicated_session"[-] will notify whenever someone tries to log in with your account from another place
	- [cyan]"permissions_change"[-] will notify whenever your permission level changes.
	- [cyan]"all"[-] subscribes to every hook mentioned before
	
[yellow::b]/unsubscribe[-::-] [green]<hook>[-]: Unsubscribes from a specific event in the server
	- Available options are the same as for [yellow::b]/subscribe[-::-]

[yellow::b]/admin[-::-] [green]<operation>[-] [blue](...)[-]: Performs an administrative operation
	- [cyan]"shutdown <offset>"[-] will perform a shutdown in the current time + offset (in minutes)
	- [cyan]"broadcast <message>[-] will send a message to all online users of the server
	- [cyan]"ban <username>"[-] will ban the specified user from the server
	- [cyan]"kick <username>"[-] will disconnect the specified user from the server
	- [cyan]"setperms <username> <permissions>[-] will set the permission level of the new user
	- [cyan]"motd <motd>"[-] will set a new MOTD (message of the day) for the server

[yellow::b]/recover[-::-] [green]<user>[-] [blue](-cleanup)[-]: Recovers data from a dangling user
	- If a user has become dangling (server is "Unknown"), this can be used to recover its data
	- This command will only work with dangling users
	- A popup asking for the password of the account to recover will appear
	- If "-cleanup" is used, the user will be deleted from the database after recovery
`

/* MESSAGES */

// Identifies a TUI message.
type Message struct {
	Buffer    string    // Buffer to store it in
	Sender    string    // Who sends it
	Content   string    // Message text
	Timestamp time.Time // Time when it occurred
	Source    string    // Destination name
}

// Returns the TLS secondary text for servers
func tlsText(addr net.Addr, tls bool) string {
	if tls {
		return addr.String() + " (TLS)"
	}

	return addr.String()
}

// Sends a predefines message on every new server
func welcomeMessage(t *TUI) {
	s := t.Active()

	text := "You are currently in the default channel for this server.\n" +
		"Use [yellow]/connect[-] to establish connection to the server.\n" +
		"You may then use [yellow]/register[-] or [yellow]/login[-] to use an account."

	t.sendMessage(Message{
		Buffer:    defaultBuffer,
		Sender:    "System",
		Content:   text,
		Timestamp: time.Now(),
		Source:    s.Name(),
	})
}

// Sends a packet to the debug channel
func (t *TUI) debugPacket(content string) {
	l := len(content)
	t.sendMessage(Message{
		Buffer:    debugBuffer,
		Sender:    "System",
		Content:   content[:l-1],
		Timestamp: time.Now(),
		Source:    localServer, // Local server
	})
}

// Binds a function that sends System message to the server
// and buffer that are active in the moment the function was ran.
// An optional prompt params[0] and buffer params[1] can be given
func (t *TUI) systemMessage(params ...string) cmds.OutputFunc {
	buffer := t.Buffer()
	server := t.Active()

	var prompt string
	if len(params) > 0 && params[0] != "" {
		prompt = fmt.Sprintf(
			"Running [lightgray::b]%s[-::-]: ",
			params[0],
		)
	}

	if len(params) > 1 && params[1] != "" {
		buffer = params[1]
	}

	fun := func(s string, out cmds.OutputType) {
		switch out {
		case cmds.PROMPT, cmds.USRSRESPONSE, cmds.COLOR, cmds.PLAIN:
			return // Ignore these
		case cmds.PACKET:
			t.debugPacket(s)
		default:
			needVerbose := out == cmds.INTERMEDIATE || out == cmds.SECONDARY
			if needVerbose && !t.params.Verbose {
				return
			}

			if out == cmds.INFO {
				prompt = ""
			}

			t.sendMessage(Message{
				Buffer:    buffer,
				Sender:    "System",
				Content:   prompt + s,
				Timestamp: time.Now(),
				Source:    server.Name(),
			})
		}
	}

	return fun
}

// Gets all the old messages that are stored in the database and
// prints them to the buffer.
func getOldMessages(t *TUI, s Server, username string) {
	print := t.systemMessage()

	data, _ := s.Online()
	user, err := db.GetExternalUser(
		t.db,
		username,
		data.Server.Address,
		data.Server.Port,
	)
	if err != nil {
		print("failed to get old messages due to "+err.Error(), cmds.ERROR)
	}

	msgs, err := db.GetAllUsersMessages(
		t.db,
		data.LocalUser.User.Username,
		user.User.Username,
		data.Server.Address,
		data.Server.Port,
	)
	if err != nil {
		print("failed to get old messages due to "+err.Error(), cmds.ERROR)
	}

	uname := data.LocalUser.User.Username
	for _, v := range msgs {
		sender := v.SourceUser.Username
		if sender == uname {
			sender = selfSender
		}

		t.sendMessage(Message{
			Buffer:    username,
			Sender:    sender,
			Content:   v.Text,
			Timestamp: v.Stamp,
			Source:    s.Name(),
		})
	}
}

// Wrapper function for sending messages to the TUI.
// It sends the message to the server by the name of the destination
func (t *TUI) sendMessage(msg Message) {
	s, ok := t.servers.Get(msg.Source)
	if !ok {
		return
	}

	ok, err := s.Receive(msg)
	// Error on a message we sent ourselves
	if err != nil && msg.Sender == selfSender {
		t.showError(err)
		return
	}

	// If the server received it and we are
	// in the destionation buffer we render it
	if ok && t.Buffer() == msg.Buffer {
		t.renderMsg(msg)
	}
}

/* RENDERING */

// Returns the text input label with username or not
func unameLabel(uname string) string {
	if uname == "" {
		return defaultLabel
	}

	return fmt.Sprintf(" (%s)%s", uname, defaultLabel)
}

// Renders a date in screen if the last displayed
// date is on a different day.
func (t *TUI) renderDate(date time.Time) {
	ly, lm, ld := t.status.lastDate.Date()
	ry, rm, rd := date.Date()
	equal := ly == ry && lm == rm && ld == rd

	if equal {
		return
	}

	formatted := date.Format(time.DateOnly)
	fmt.Fprintf(
		t.comp.text,
		"--- %s%s%s ---\n",
		"[green::i]", formatted, "[-::-]",
	)
	t.status.lastDate = date
}

// Renders a message in the screen by previously
// rendering the date. Uses text formatting.
func (t *TUI) renderMsg(msg Message) {
	if msg.Sender == "" {
		fmt.Fprintf(t.comp.text, "%s", msg.Content)
		t.comp.text.ScrollToEnd()
		return
	}

	t.renderDate(msg.Timestamp)
	format := time.Kitchen // Just time, not date

	// Align with the previous line
	pad := strings.Repeat(" ", len(msg.Sender))

	// Replaces newlines with padding only until last newline
	n := strings.Count(msg.Content, "\n")
	content := strings.Replace(msg.Content, "\n", "\n\t\t\t   "+pad, n)

	f := msg.Timestamp.Format(format)
	color := "[blue::b]"
	if msg.Sender == selfSender {
		color = "[yellow::b]"
	}
	if msg.Sender == "System" {
		color = "[purple::b]"
	}

	_, err := fmt.Fprintf(
		t.comp.text,
		"[%s%s%s] at %s%07s%s: %s\n",
		color, msg.Sender, "[-::-]",
		"[gray::u]", f, "[-::-]",
		content,
	)

	if err != nil {
		t.showError(err)
		return
	}

	t.comp.text.ScrollToEnd()
}

// Displays or hides the help window by also showing
// or hiding the input.
func (t *TUI) toggleHelp() {
	if !t.status.showingHelp {
		t.status.showingHelp = true
		t.comp.text.Clear()
		t.area.bottom.ResizeItem(t.comp.input, 0, 0)
		t.comp.text.SetTitle("Help")

		fmt.Fprint(t.comp.text, KeybindHelp[1:])
		fmt.Fprint(t.comp.text, "\n")
		fmt.Fprint(t.comp.text, CommandHelp[1:])

		t.comp.text.ScrollToBeginning()
	} else {
		t.status.showingHelp = false
		t.area.bottom.ResizeItem(t.comp.input, inputSize, 0)
		t.comp.text.SetTitle("Messages")
		t.renderBuffer(t.Active().Buffers().current)
	}
}

// Displays an error in the error bar temporarily.
func (t *TUI) showError(err error) {
	t.comp.errors.Clear()
	t.area.bottom.ResizeItem(t.comp.errors, errorSize, 0)
	fmt.Fprintf(t.comp.errors, " [red]Error: %s![-:-]", err)

	go func() {
		<-time.After(time.Duration(errorMessage) * time.Second)
		t.comp.errors.Clear()
		t.area.bottom.ResizeItem(t.comp.errors, 0, 0)
	}()
}
