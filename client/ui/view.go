package ui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type tab struct {
	messages models.Slice[Message]
	name     string
	system   bool
}

type opts struct {
	showUsers   bool
	showBufs    bool
	creatingBuf bool
}

type TUI struct {
	area   areas
	comp   components
	tabs   models.Table[string, *tab]
	config opts
	active string
}

func (t *TUI) tabPopup(app *tview.Application) {
	t.config.creatingBuf = true
	input := tview.NewInputField().
		SetLabel("Enter buffer name: ").
		SetFieldBackgroundColor(tcell.ColorDefault)
	input.SetBorder(false).
		SetBackgroundColor(tcell.ColorDefault)
	t.area.chat.ResizeItem(t.comp.input, 0, 0)
	t.area.chat.AddItem(input, 1, 0, true)
	app.SetFocus(input)

	input.SetDoneFunc(func(key tcell.Key) {
		text := input.GetText()
		if text == "" {
			return
		}

		t.newTab(text, false)
		t.area.chat.RemoveItem(input)
		t.area.chat.ResizeItem(t.comp.input, inputSize, 0)
		app.SetFocus(t.comp.input)
		t.config.creatingBuf = false
	})
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

func (t *TUI) newTab(name string, system bool) {
	tab := &tab{
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	s := strconv.Itoa(t.tabs.Len() + 1)
	r := []rune(s)

	t.comp.buffers.AddItem(name, "", r[0], nil)
	t.tabs.Add(name, tab)
}
