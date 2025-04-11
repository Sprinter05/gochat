package ui

import (
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	chat    *tview.TextView   = tview.NewTextView()
	buffers *tview.List       = tview.NewList()
	users   *tview.List       = tview.NewList()
	input   *tview.InputField = tview.NewInputField()
)

func init() {
	chat.SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Messages")
	buffers.SetBorder(true).
		SetTitle("Buffers").
		SetBackgroundColor(tcell.ColorDefault)
	users.SetBorder(true).
		SetTitle("Users").
		SetBackgroundColor(tcell.ColorDefault)
	input.SetLabelColor(tcell.ColorDefault).
		SetLabel("").
		SetFieldBackgroundColor(tcell.ColorDefault).
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

func newTab(name string) *tab {
	buffers.AddItem(name, "", 0, nil)

	return &tab{
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   false,
	}
}
