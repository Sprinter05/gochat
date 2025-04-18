package ui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sprinter05/gochat/internal/models"
)

type tab struct {
	messages models.Slice[Message]
	name     string
	system   bool
}

type opts struct {
	showUsers bool
	showBufs  bool
}

type TUI struct {
	area   areas
	comp   components
	tabs   models.Table[string, *tab]
	config opts
	active string
}

func (t *TUI) systemTab() {
	t.newTab("System", true)
	t.active = "System"

	fmt.Fprint(t.comp.text, Logo)

	t.SendMessage("System", Message{
		Sender:    "System",
		Content:   "Welcome to gochat!",
		Timestamp: time.Now(),
	})
}

func (t *TUI) newTab(name string, system bool) *tab {
	tab := &tab{
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	s := strconv.Itoa(t.tabs.Len() + 1)
	r := []rune(s)

	t.comp.buffers.AddItem(name, "", r[0], nil)
	t.tabs.Add(name, tab)
	return tab
}
