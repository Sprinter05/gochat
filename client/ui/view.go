package ui

import (
	"time"

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

type TUI struct {
	Area   *tview.Flex
	tabs   models.Table[string, *tab]
	active string
}

func (t *TUI) Init() {
	t.Area.SetBackgroundColor(tcell.ColorDefault)
	system := &tab{
		messages: models.NewSlice[Message](0),
		name:     "System",
		system:   true,
	}

	t.tabs.Add(system.name, system)
	t.active = system.name

	buffers.AddItem(system.name, "", 0, nil)
	t.SendMessage("System", Message{
		Sender:    "System",
		Content:   "Welcome to gochat!",
		Timestamp: time.Now(),
	})

	t.Area.ResizeItem(users, 0, 0)
}

func NewTab(name string) *tab {
	return &tab{
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   false,
	}
}

func NewTUI() *TUI {
	return &TUI{
		tabs: models.NewTable[string, *tab](0),
		Area: tview.NewFlex().
			AddItem(buffers, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(chat, 0, 1, false).
					AddItem(input, 2, 0, true),
				0, 5, true,
			).AddItem(users, 0, 1, false),
	}
}
