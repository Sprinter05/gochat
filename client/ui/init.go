package ui

import (
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (t *TUI) systemTab() {
	t.area.SetBackgroundColor(tcell.ColorDefault)
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
}

func (t *TUI) newApp() *tview.Application {
	app := tview.NewApplication().SetRoot(t.area, true).SetFocus(t.area)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlU:
			if t.config.showUsers {
				t.area.ResizeItem(users, 0, 0)
				t.config.showUsers = false
			} else {
				t.area.ResizeItem(users, 0, 1)
				t.config.showUsers = true
			}
		case tcell.KeyCtrlB:
			if t.config.showUsers {
				t.area.ResizeItem(buffers, 0, 0)
				t.config.showBufs = false
			} else {
				t.area.ResizeItem(buffers, 0, 1)
				t.config.showBufs = true
			}
		}
		return event
	})
	return app
}

func NewTUI() (*TUI, *tview.Application) {
	t := TUI{
		tabs: models.NewTable[string, *tab](0),
		area: tview.NewFlex().
			AddItem(buffers, 0, 1, false).
			AddItem(
				tview.NewFlex().SetDirection(tview.FlexRow).
					AddItem(chat, 0, 1, false).
					AddItem(input, 2, 0, true),
				0, 5, true,
			).AddItem(users, 0, 0, false),
		config: opts{
			showUsers: false,
			showBufs:  true,
		},
	}

	t.systemTab()
	app := t.newApp()

	return &t, app
}
