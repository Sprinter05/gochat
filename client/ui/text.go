package ui

import (
	"fmt"
	"time"
)

type Message struct {
	Sender    string
	Content   string
	Timestamp time.Time
}

func (m Message) Render() string {
	now := time.Now()
	format := time.Kitchen
	if now.Sub(m.Timestamp) > (time.Hour * 24 * 365) {
		format = time.DateTime
	} else if now.Sub(m.Timestamp) > (time.Hour * 24) {
		format = time.Stamp
	}

	t := m.Timestamp.Format(format)
	color := "[blue::b]"
	if m.Sender == selfSender {
		color = "[yellow::b]"
	}

	return fmt.Sprintf(
		"[%s] at %s: %s\n",
		color+m.Sender+"[-:-:-:-]",
		"[gray::u]"+t+"[-:-:-:-]",
		m.Content,
	)
}

// might not disappear after 3 seconds if no input is received
func (t *TUI) ShowError(err error) {
	t.comp.errors.Clear()
	t.area.chat.ResizeItem(t.comp.errors, 0, 1)
	fmt.Fprintf(t.comp.errors, " [red]Error: %s![-:-:-:-]", err)
	go func() {
		<-time.After(time.Duration(errorMessage) * time.Second)
		t.area.chat.ResizeItem(t.comp.errors, 0, 0)
	}()
}

func (t *TUI) SendMessage(buf string, msg Message) {
	b, ok := t.tabs.Get(buf)
	if !ok {
		t.newTab(buf, false)
		b, _ = t.tabs.Get(buf)
	}

	if b.system && msg.Sender == selfSender {
		t.ShowError(ErrorSystemBuf)
		return
	}

	b.messages.Add(msg)

	if buf == t.active {
		fmt.Fprint(t.comp.text, msg.Render())
	}
}
