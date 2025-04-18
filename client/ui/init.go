package ui

import (
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const Logo string = `
                   _           _   
                  | |         | |  
   __ _  ___   ___| |__   __ _| |_ 
  / _  |/ _ \ / __| '_ \ / _  | __|
 | (_| | (_) | (__| | | | (_| | |_ 
  \__  |\___/ \___|_| |_|\____|\__|
   __/ |                           
  |___/   

`

const (
	self = "You"
)

type areas struct {
	main *tview.Flex
	chat *tview.Flex
}

type components struct {
	text    *tview.TextView
	buffers *tview.List
	users   *tview.List
	input   *tview.TextArea
}

func (t *TUI) setupLayout() {
	t.comp.text.
		SetDynamicColors(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true).
		SetTitle("Messages")
	t.comp.buffers.
		SetSelectedStyle(tcell.StyleDefault.Underline(true)).
		SetSelectedTextColor(tcell.ColorPurple).
		ShowSecondaryText(false).
		SetBorder(true).
		SetTitle("Buffers").
		SetBackgroundColor(tcell.ColorDefault)
	t.comp.users.
		SetBorder(true).
		SetTitle("Users").
		SetBackgroundColor(tcell.ColorDefault)
	t.comp.input.
		SetLabel(" > ").
		SetPlaceholder("Write here...").
		SetWrap(true).
		SetWordWrap(true).
		SetBorder(true).
		SetBackgroundColor(tcell.ColorDefault)
}

func (t *TUI) setupKeybinds(app *tview.Application) {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlU:
			if t.config.showUsers {
				t.area.main.ResizeItem(t.comp.users, 0, 0)
				t.config.showUsers = false
			} else {
				t.area.main.ResizeItem(t.comp.users, 0, 1)
				t.config.showUsers = true
			}
		case tcell.KeyCtrlB:
			if t.config.showBufs {
				t.area.main.ResizeItem(t.comp.buffers, 0, 0)
				t.config.showBufs = false
			} else {
				t.area.main.ResizeItem(t.comp.buffers, 0, 2)
				t.config.showBufs = true
			}
		case tcell.KeyCtrlR:
			app.Sync()
		}
		return event
	})

	t.comp.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLF:
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		case tcell.KeyEnter:
			if t.comp.input.GetText() == "" {
				return nil
			}
			t.SendMessage(t.active, Message{
				Sender:    self,
				Content:   t.comp.input.GetText(),
				Timestamp: time.Now(),
			})
			t.comp.input.SetText("", false)
			return nil
		}
		return event
	})
}

func New() (*TUI, *tview.Application) {
	comps := components{
		text:    tview.NewTextView(),
		buffers: tview.NewList(),
		users:   tview.NewList(),
		input:   tview.NewTextArea(),
	}

	chat := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.text, 0, 1, false).
		AddItem(comps.input, 4, 0, true)
	chat.SetBackgroundColor(tcell.ColorDefault)

	main := tview.NewFlex().
		AddItem(comps.buffers, 0, 2, false).
		AddItem(chat, 0, 6, true).
		AddItem(comps.users, 0, 0, false)
	main.SetBackgroundColor(tcell.ColorDefault)

	t := TUI{
		tabs: models.NewTable[string, *tab](0),
		comp: comps,
		area: areas{
			main: main,
			chat: chat,
		},
		config: opts{
			showUsers: false,
			showBufs:  true,
		},
	}

	app := tview.NewApplication().SetRoot(t.area.main, true).SetFocus(t.area.main)
	t.setupLayout()
	t.setupKeybinds(app)
	t.systemTab()

	return &t, app
}
