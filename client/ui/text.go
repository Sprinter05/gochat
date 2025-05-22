package ui

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/spec"
)

/* TEXT */

const KeybindHelp string = `
[-::u]Keybinds Manual:[-::-]

[yellow::b]Ctrl-Alt-H/Ctrl-Shift-H[-::-]: Show/Hide help window

[yellow::b]Ctrl-Q[-::-]: Exit program

[yellow::b]Ctrl-T[-::-]: Focus chat/input window
	- In the [-::b]chat window[-::-] use [green]Up/Down[-::-] to move
	- In the [-::b]input window[-::-] use [green]Escape[-::-] to clear the text
	- In the [-::b]input window[-::-] use [green]Alt-Enter/Shift-Enter[-::-] to add a newline
	- In the [-::b]input window[-::-] use [green]Up[-::-] to browse through the history of commands ran.

[yellow::b]Ctrl-K + Ctrl-N[-::-]: Create a new buffer
	- [green]Esc[-::-] to cancel
	- [green]Enter[-::-] to confirm

[yellow::b]Ctrl-K + Ctrl-W/Ctrl-H[-::-]: Hide currently focused buffer
	- It can be shown again by creating a buffer with the same name
	
[yellow::b]Ctrl-K + Ctrl-X[-::-]: Delete currently focused buffer
	- This will permanantely delete all messages if the buffer corresponded to a remote user

[yellow::b]Ctrl-K[-::-] + [green::b]1-z[-::-]: Jump to specific buffer
	- Press [green]Esc[-::-] to cancel the jump

[yellow::b]Ctrl-S + Ctrl-N[-::-]: Create a new server
	- [green]Esc[-::-] to cancel
	- [green]Enter[-::-] to confirm the different steps
	
[yellow::b]Ctrl-S + Ctrl-W/Ctrl-H[-::-]: Hide currently focused server
	- It can be shown again under any name by typing the same address it was used at creation

[yellow::b]Ctrl-S + Ctrl-X[-::-]: Delete currently focused server
	- This will permanantely delete all asocciated data

[yellow::b]Ctrl-S[-::-] + [green::b]1-9[-::-]: Jump to specific server
	- Press [green]Esc[-::-] to cancel the jump
	
[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Ctrl-B[-::-]: Show/Hide buffer list

[yellow::b]Ctrl-U[-::-]: Show/Hide user list

[yellow::b]Ctrl-R[-::-]: Redraw screen
`

const CommandHelp string = `
[-::u]Commands Manual:[-::-]

[yellow::b]/version[-::-]: Displays the current version of the client and protocol

[yellow::b]/buffers[-::-]: Displays a list of all buffers in the current server
	- Those that have been hidden will also be displayed
	
[yellow::b]/tls[-::-] [green]<on/off>[-]: Enables or disables TLS connections

[yellow::b]/connect[-::-] [green](-noverify)[-]: Connects to the currently active server using its address
	- This will fail if the server is local
	- If the connection is TLS and noverify is used, certificates will not be checked

[yellow::b]/register[-::-] [green]<username>[-]: Creates a new account in the currently active server
	- A popup asking for a password to register will show up when creating a new account
	- No two accounts with the same name can exist in one single server
	- You need an active connection to use this command
	
[yellow::b]/login[-::-] [green]<username>[-]: Tries to login in the server with an account
	- A popup asking for the password asocciated to the account will show up
	- You need an active connection to use this command

[yellow::b]/logout[-::-]: Logs out of your account in the currently active server
	- You need an active connection to use this command

[yellow::b]/disconnect[-::-]: Interrumps the connection with the currently active server
	- You need an active connection to use this command

[yellow::b]/users[-::-] [green]<local/online/all>[-]: Shows a list of users according to the specified filter
	- Local will display accounts created for this server in this client
	- Online will display all connected accounts in the server
	- All will display all accounts registered in the server
	- You need an active connection to use this command unless you are displaying local users
	
[yellow::b]/request[-::-]: Attempts to manually obtain user data on the current buffer
	- This process is already done automatically if connected and logged in
`

/* MESSAGES */

// Identifies a TUI message.
type Message struct {
	Buffer    string    // Buffer to store it in
	Sender    string    // Who sends it
	Content   string    // Message text
	Timestamp time.Time // Time when it occurred
	Source    net.Addr  // Destination server
}

func welcomeMessage(t *TUI) {
	s := t.Active()

	text := "You are currently in the default channel for this server.\n" +
		"Use [yellow]/connect[-] to establish connection to the server.\n" +
		"You may then use [yellow]/register[-] or [yellow]/login[-] to use an account."

	t.SendMessage(Message{
		Buffer:    defaultBuffer,
		Sender:    "System",
		Content:   text,
		Timestamp: time.Now(),
		Source:    s.Source(),
	})
}

// Sends a packet to the debug channel
func (t *TUI) debugPacket(content string) {
	l := len(content)
	t.SendMessage(Message{
		Buffer:    debugBuffer,
		Sender:    "System",
		Content:   content[:l-1],
		Timestamp: time.Now(),
		Source:    nil, // Local server
	})
}

// Binds a function that sends System message to the server
// and buffer that are active in the moment the function was ran.
// An optional prompt params[0] and buffer params[1] can be given
func (t *TUI) systemMessage(params ...string) func(string, cmds.OutputType) {
	buffer := t.Buffer()
	server := t.Active().Source()

	var prompt string
	if len(params) > 0 {
		prompt = fmt.Sprintf(
			"Running [lightgray::b]%s[-::-]: ",
			params[0],
		)
	}

	if len(params) > 1 {
		buffer = params[1]
	}

	fun := func(s string, out cmds.OutputType) {
		switch out {
		case cmds.PROMPT, cmds.USRS:
			return
		case cmds.PACKET:
			t.debugPacket(s)
		default:
			t.SendMessage(Message{
				Buffer:    buffer,
				Sender:    "System",
				Content:   prompt + s,
				Timestamp: time.Now(),
				Source:    server,
			})
		}
	}

	return fun
}

// Sends a message to the remote connection if possible
func (t *TUI) remoteMessage(content string) {
	print := t.systemMessage("message")

	s := t.Active()
	tab := s.Buffers().Current()

	data, ok := s.Online()

	if tab != nil && tab.system {
		return
	}

	if data == nil {
		return
	}

	if tab == nil || !ok || !tab.connected {
		print("failed to send message: "+ErrorNoRemoteUser.Error(), cmds.ERROR)
		return
	}

	cmd := cmds.Command{
		Output: func(text string, outputType cmds.OutputType) {},
		Static: &t.data,
		Data:   data,
	}

	ctx, cancel := timeout()
	defer cancel()
	r := cmds.Msg(ctx, cmd, []byte(tab.name), []byte(content))
	if r.Error != nil {
		print("failed to send message: "+r.Error.Error(), cmds.ERROR)
	}
}

// Waits for new messages to be sent to that user
func (t *TUI) receiveMessages(s Server) {
	defer s.Buffers().Offline()
	data, _ := s.Online()
	print := t.systemMessage("reciv", defaultBuffer)

	for {
		if !data.IsUserLoggedIn() || !data.IsConnected() {
			// Stop listening
			return
		}

		cmd := data.Waitlist.Get(
			context.Background(), // TODO
			cmds.Find(spec.NullID, spec.RECIV),
		)

		ctx, cancel := timeout()
		msg, err := cmds.StoreReciv(
			ctx, cmd,
			cmds.Command{
				Output: func(text string, outputType cmds.OutputType) {},
				Static: &t.data,
				Data:   data,
			},
		)
		cancel()

		if err != nil {
			print(err.Error(), cmds.ERROR)
			continue
		}

		t.SendMessage(Message{
			Buffer:    msg.Sender,
			Sender:    msg.Sender,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Source:    s.Source(),
		})
	}

}

// Wrapper function for sending messages to the TUI.
// It sends the message to all servers assuming only
// the corresponding one will do something with it by
// checking the source of the messages.
func (t *TUI) SendMessage(msg Message) {
	list := t.servers.GetAll()
	for _, v := range list {
		// Each server will handle if its for them
		ok, err := v.Receive(msg)
		if err != nil && msg.Sender == selfSender {
			t.showError(err)
			return
		}

		// If the server received it and we are
		// in the destionation buffer we render it
		if ok && t.Buffer() == msg.Buffer {
			t.renderMsg(msg)
			break
		}
	}
}

/* RENDERING */

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
	t.area.bottom.ResizeItem(t.comp.errors, 0, 1)
	fmt.Fprintf(t.comp.errors, " [red]Error: %s![-:-]", err)

	go func() {
		<-time.After(time.Duration(errorMessage) * time.Second)
		t.comp.errors.Clear()
		t.area.bottom.ResizeItem(t.comp.errors, 0, 0)
	}()
}
