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
	if m.Sender == self {
		color = "[yellow::b]"
	}

	return fmt.Sprintf(
		"[%s] at %s: %s\n",
		color+m.Sender+"[-:-:-:-]",
		"[gray::u]"+t+"[-:-:-:-]",
		m.Content,
	)
}

func (t *TUI) SendMessage(buf string, msg Message) {
	b, ok := t.tabs.Get(buf)
	if !ok {
		t.newTab(buf, false)
		b, _ = t.tabs.Get(buf)
	}

	b.messages.Add(msg)

	if buf == t.active {
		fmt.Fprint(t.comp.chat, msg.Render())
	}
}
