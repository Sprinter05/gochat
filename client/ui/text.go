package ui

import (
	"fmt"
	"strings"
	"time"
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

	content := strings.Replace(msg.Content, "\n", "\n\t\t\t\t  ", -1)

	f := msg.Timestamp.Format(format)
	color := "[blue::b]"
	if msg.Sender == selfSender {
		color = "[yellow::b]"
	}

	_, err := fmt.Fprintf(
		t.comp.text,
		"[%s%s%s] at %s%07s%s: %s\n",
		color, msg.Sender, "[-::-]",
		"[darkgray::u]", f, "[-::-]",
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
	check := strings.Replace(msg.Content, "\n", "", -1)
	if check == "" {
		return
	}

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
