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
	t := m.Timestamp.Format(time.Kitchen)
	return fmt.Sprintf(
		"[%s] at %s: %s\n",
		"[blue::b]"+m.Sender+"[-:-:-:-]",
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
		fmt.Fprint(chat, msg.Render())
	}
}
