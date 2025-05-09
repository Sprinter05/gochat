package ui

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Message struct {
	Buffer    string
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
	if msg.Sender == "" {
		fmt.Fprintf(t.comp.text, "%s", msg.Content)
		t.comp.text.ScrollToEnd()
		return
	}

	t.renderDate(msg.Timestamp)
	format := time.Kitchen

	pad := strings.Repeat(" ", len(msg.Sender))

	// removes only until last newline
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
	}

	t.comp.text.ScrollToEnd()
}

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

// Assumes buffer list is already changed
func (t *TUI) renderBuffer(buf string) {
	b, ok := t.Active().Buffers().tabs.Get(buf)
	if !ok {
		return
	}

	t.Active().Buffers().current = buf

	if b.system {
		t.comp.buffers.SetSelectedTextColor(tcell.ColorPlum)
	} else {
		t.comp.buffers.SetSelectedTextColor(tcell.ColorPurple)
	}

	if t.status.showingHelp {
		return
	}

	t.comp.text.Clear()
	msgs := t.Active().Messages(buf)
	for _, v := range msgs {
		t.renderMsg(v)
	}
}

func (t *TUI) SendMessage(msg Message) {
	list := t.servers.GetAll()
	for _, v := range list {
		// Each server will handle if its for them
		ok, err := v.Receive(msg)
		if err != nil {
			t.showError(err)
			return
		}

		if ok {
			if v.Buffers().current == msg.Buffer {
				t.renderMsg(msg)
			}
			break
		}
	}
}

func (t *TUI) systemMessage() func(string) {
	buffer := t.Buffer()
	server := t.Active().Source()
	fun := func(s string) {
		t.SendMessage(Message{
			Buffer:    buffer,
			Sender:    "System",
			Content:   s,
			Timestamp: time.Now(),
			Source:    server,
		})
	}

	return fun
}
