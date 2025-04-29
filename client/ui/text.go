package ui

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Message struct {
	Sender    string
	Content   string
	Timestamp time.Time
	Source    net.Addr
}

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

func (t *TUI) renderMsg(msg Message) {
	t.renderDate(msg.Timestamp)
	format := time.Kitchen

	pad := strings.Repeat(" ", len(msg.Sender))
	content := strings.Replace(msg.Content, "\n", "\n\t\t\t   "+pad, -1)

	f := msg.Timestamp.Format(format)
	color := "[blue::b]"
	if msg.Sender == selfSender {
		color = "[yellow::b]"
	}

	_, err := fmt.Fprintf(
		t.comp.text,
		"[%s%s%s] at %s%07s%s: %*s\n",
		color, msg.Sender, "[-::-]",
		"[gray::u]", f, "[-::-]",
		len(msg.Sender), content,
	)

	if err != nil {
		t.showError(err)
	}

	t.comp.text.ScrollToEnd()
}

func (t *TUI) toggleHelp() {
	if !t.status.showingHelp {
		t.status.showingHelp = true
		t.comp.text.Clear()
		t.area.chat.ResizeItem(t.comp.input, 0, 0)
		t.comp.text.SetTitle("Help")
		fmt.Fprint(t.comp.text, Help[1:])
		t.comp.text.ScrollToBeginning()
	} else {
		t.status.showingHelp = false
		t.area.chat.ResizeItem(t.comp.input, inputSize, 0)
		t.comp.text.SetTitle("Messages")
		t.ChangeBuffer(t.active)
	}
}

func (t *TUI) showError(err error) {
	t.comp.errors.Clear()
	t.area.chat.ResizeItem(t.comp.errors, 0, 1)
	fmt.Fprintf(t.comp.errors, " [red]Error: %s![-:-:-:-]", err)

	go func() {
		<-time.After(time.Duration(errorMessage) * time.Second)
		t.comp.errors.Clear()
		t.area.chat.ResizeItem(t.comp.errors, 0, 0)
	}()
}

// Assumes buffer list is already changed
func (t *TUI) ChangeBuffer(buf string) {
	t.active = buf

	if t.status.showingHelp {
		return
	}

	msgs := t.Active().Messages(buf)
	for _, v := range msgs {
		t.renderMsg(v)
	}
}

func (t *TUI) SendMessage(buf string, msg Message) {
	list := t.servers.GetAll()
	for _, v := range list {
		// Each server will handle if its for them
		v.Receive(msg)
	}

	// TODO: render message
}
