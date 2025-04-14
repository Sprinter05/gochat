package ui

import (
	"strconv"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	chat    *tview.TextView = tview.NewTextView()
	buffers *tview.List     = tview.NewList()
	users   *tview.List     = tview.NewList()
	input   *tview.TextArea = tview.NewTextArea()
)

func init() {
	chat.
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Messages")
	buffers.
		SetSelectedStyle(tcell.StyleDefault.Underline(true)).
		SetSelectedTextColor(tcell.ColorPurple).
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("Buffers").
		SetBackgroundColor(tcell.ColorDefault)
	users.
		SetBorder(true).
		SetTitle("Users").
		SetBackgroundColor(tcell.ColorDefault)
	input.
		SetLabel(" > ").
		SetPlaceholder("Write here...").
		SetBorder(true).
		SetBackgroundColor(tcell.ColorDefault)
}

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
	area   *tview.Flex
	tabs   models.Table[string, *tab]
	config opts
	active string
}

func (t *TUI) newTab(name string, system bool) *tab {
	tab := &tab{
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	s := strconv.Itoa(t.tabs.Len() + 1)
	r := []rune(s)

	buffers.AddItem(name, "", r[0], nil)
	t.tabs.Add(name, tab)
	return tab
}
