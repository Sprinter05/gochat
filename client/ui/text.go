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

func (t *TUI) Render(msg Message) {
	now := time.Now()
	format := time.Kitchen
	if now.Sub(msg.Timestamp) > (time.Hour * 24 * 365) {
		format = time.DateTime
	} else if now.Sub(msg.Timestamp) > (time.Hour * 24) {
		format = time.Stamp
	}

	f := msg.Timestamp.Format(format)
	color := "[blue::b]"
	if msg.Sender == selfSender {
		color = "[yellow::b]"
	}

	s := fmt.Sprintf(
		"[%s] at %s: %s\n",
		color+msg.Sender+"[-:-:-:-]",
		"[gray::u]"+f+"[-:-:-:-]",
		msg.Content,
	)

	fmt.Fprint(t.comp.text, s)
}

func (t *TUI) ChangeBuf(buf string) {
	t.active = buf
	b, ok := t.tabs.Get(buf)
	if !ok {
		panic("non created tab when selecting buffer")
	}

	t.comp.text.Clear()
	msgs := b.messages.Copy(0)
	for _, v := range msgs {
		t.Render(v)
	}
}

// might not disappear after 3 seconds if no input is received
// force redraw??
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
		t.Render(msg)
	}
}
