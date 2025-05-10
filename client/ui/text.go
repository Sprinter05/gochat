package ui

import (
	"fmt"
	"net"
	"strings"
	"time"
)

/* MESSAGES */

// Identifies a TUI message.
type Message struct {
	Buffer    string    // Buffer to store it in
	Sender    string    // Who sends it
	Content   string    // Message text
	Timestamp time.Time // Time when it occurred
	Source    net.Addr  // Destination server
}

// Binds a function that sends System message to the server
// and buffer that are active in the moment the function was ran.
// An optional prompt for the messages can be given.
func (t *TUI) systemMessage(command ...string) func(string) {
	buffer := t.Buffer()
	server := t.Active().Source()

	var prompt string
	if len(command) != 0 {
		prompt = fmt.Sprintf(
			"Running [lighrgray::b]%s[-::-]: ",
			command[0],
		)
	}

	fun := func(s string) {
		t.SendMessage(Message{
			Buffer:    buffer,
			Sender:    "System",
			Content:   prompt + s,
			Timestamp: time.Now(),
			Source:    server,
		})
	}

	return fun
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
		if ok {
			if v.Buffers().current == msg.Buffer {
				t.renderMsg(msg)
			}
			break
		}
	}
}

/* RENDERING */

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
		fmt.Fprint(t.comp.text, Help[1:])
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
