package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

type Message struct {
	Sender    string
	Content   string
	Timestamp time.Time
}

func (t *TUI) renderMsg(msg Message) {
	format := time.Kitchen
	if time.Since(msg.Timestamp) > (time.Hour * 24 * 365) {
		format = time.DateTime
	} else if time.Since(msg.Timestamp) > (time.Hour * 24) {
		format = time.Stamp
	}

	f := msg.Timestamp.Format(format)
	color := "[blue::b]"
	if msg.Sender == selfSender {
		color = "[yellow::b]"
	}

	s := fmt.Sprintf(
		"[%s] at %s: %s\n",
		color+msg.Sender+"[-::-]",
		"[gray::u]"+f+"[-::-]",
		msg.Content,
	)

	fmt.Fprint(t.comp.text, s)
	t.comp.text.ScrollToEnd()
}

func (t *TUI) toggleHelp() {
	if !t.status.showingHelp {
		t.comp.text.Clear()
		t.area.chat.ResizeItem(t.comp.input, 0, 0)
		fmt.Fprint(t.comp.text, Help[1:])
		t.comp.text.ScrollToBeginning()
		t.comp.buffers.SetSelectedTextColor(tcell.ColorGrey)
		t.comp.text.SetTitle("Help")
		t.status.showingHelp = true
	} else {
		t.area.chat.ResizeItem(t.comp.input, inputSize, 0)
		t.comp.buffers.SetSelectedTextColor(tcell.ColorPurple)
		t.comp.text.SetTitle("Messages")
		t.status.showingHelp = false
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

	b, ok := t.tabs.Get(buf)
	if !ok {
		panic("non created tab when selecting buffer")
	}

	t.comp.text.Clear()
	if b.system {
		fmt.Fprint(t.comp.text, Logo[1:])
	}

	msgs := b.messages.Copy(0)
	for _, v := range msgs {
		t.renderMsg(v)
	}
}

func (t *TUI) SendMessage(buf string, msg Message) {
	b, ok := t.tabs.Get(buf)
	if !ok {
		t.newTab(buf, false)
		b, _ = t.tabs.Get(buf)
	}

	if b.system && msg.Sender == selfSender {
		t.showError(ErrorSystemBuf)
		return
	}

	b.messages.Add(msg)

	if buf == t.active && !t.status.showingHelp {
		t.renderMsg(msg)
	}
}
