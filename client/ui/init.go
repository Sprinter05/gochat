package ui

import (
	"errors"
	"fmt"
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TODO: selecting non existing buffer

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

const Help string = `
[-::u]The gochat Instructions Manual:[-::-]

[yellow::b]Ctrl-Shift-H[-::-]: Show/Hide help window

[yellow::b]Ctrl-Q[-::-]: Exit program

[yellow::b]Ctrl-T[-::-]: Focus chat/input window
	- In the [-::b]chat window[-::-] use [green]Up/Down[-::-] to move
	- In the [-::b]input window[-::-] use [green]Ctrl-Enter[-::-] to add a newline

[yellow::b]Ctrl-N[-::-]: Create a new buffer
	- [green]Esc[-::-] to cancel
	- [green]Enter[-::-] to confirm

[yellow::b]Ctrl-X[-::-]: Remove currenly focused buffer

[yellow::b]Ctrl-K[-::-] + [green::b]1-z[-::-]: Jump to specific buffer

[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Ctrl-B[-::-]: Show/Hide buffer list

[yellow::b]Ctrl-U[-::-]: Show/Hide user list

[yellow::b]Ctrl-R[-::-]: Redraw screen
`

const (
	selfSender     string = "You"
	inputSize      int    = 4
	errorMessage   uint   = 3    // seconds
	asciiNumbers   int    = 0x30 // Start of ASCII for number 1
	asciiLowercase int    = 0x61 // Start of ASCII for lowercase a
)

var (
	ErrorSystemBuf = errors.New("performing action on system buffer")
	ErrorNoText    = errors.New("no text has been given")
	ErrorExists    = errors.New("item already exists")
	ErrorNotFound  = errors.New("item does not exist")
	ErrorMaxBufs   = errors.New("maximum amount of buffers reached")
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
	errors  *tview.TextView
}

func setupLayout() (areas, components) {
	comps := components{
		text:    tview.NewTextView(),
		buffers: tview.NewList(),
		users:   tview.NewList(),
		input:   tview.NewTextArea(),
		errors:  tview.NewTextView(),
	}

	chat := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.text, 0, 30, false).
		AddItem(comps.errors, 0, 0, false).
		AddItem(comps.input, inputSize, 0, true)
	chat.SetBackgroundColor(tcell.ColorDefault)

	main := tview.NewFlex().
		AddItem(comps.buffers, 0, 2, false).
		AddItem(chat, 0, 6, true).
		AddItem(comps.users, 0, 0, false)
	main.SetBackgroundColor(tcell.ColorDefault)

	areas := areas{
		main: main,
		chat: chat,
	}

	return areas, comps
}

func setupStyle(t *TUI) {
	t.comp.text.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetScrollable(true).
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

	t.comp.errors.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(false)
}

func setupHandlers(t *TUI, app *tview.Application) {
	t.comp.buffers.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		t.ChangeBuffer(s1)
		if t.status.showingHelp {
			app.SetFocus(t.comp.text)
		} else {
			app.SetFocus(t.comp.input)
		}
	})

	t.comp.errors.SetChangedFunc(func() {
		app.Draw()
	})
}

func setupInput(t *TUI) {
	t.comp.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLF:
			return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
		case tcell.KeyEnter:
			if t.comp.input.GetText() == "" {
				return nil
			}

			t.SendMessage(t.active, Message{
				Sender:    selfSender,
				Content:   t.comp.input.GetText(),
				Timestamp: time.Now(),
			})

			t.comp.input.SetText("", false)
			return nil
		}
		return event
	})
}

func setupKeybinds(t *TUI, app *tview.Application) {
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlQ:
			app.Stop()
		case tcell.KeyCtrlC:
			return nil
		case tcell.KeyCtrlU:
			if t.status.showingUsers {
				t.area.main.ResizeItem(t.comp.users, 0, 0)
				t.status.showingUsers = false
			} else {
				t.area.main.ResizeItem(t.comp.users, 0, 1)
				t.status.showingUsers = true
			}
		case tcell.KeyCtrlB:
			if t.status.showingBufs {
				t.area.main.ResizeItem(t.comp.buffers, 0, 0)
				t.status.showingBufs = false
			} else {
				t.area.main.ResizeItem(t.comp.buffers, 0, 2)
				t.status.showingBufs = true
			}
		case tcell.KeyCtrlT:
			if t.status.creatingBuf || t.status.showingHelp {
				break
			}

			if !t.comp.text.HasFocus() {
				app.SetFocus(t.comp.text)
				return nil
			} else {
				app.SetFocus(t.comp.input)
				return nil
			}
		case tcell.KeyCtrlH:
			if event.Modifiers()&tcell.ModShift == tcell.ModShift {
				app.SetFocus(t.comp.text)
				t.toggleHelp()
				if !t.status.showingHelp {
					app.SetFocus(t.comp.input)
				}
			}
		case tcell.KeyCtrlK:
			if t.status.creatingBuf {
				break
			}

			if !t.comp.buffers.HasFocus() {
				app.SetFocus(t.comp.buffers)
				return nil
			}
		case tcell.KeyDown:
			if t.status.creatingBuf {
				break
			}

			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeTab(curr + 1)
			}
		case tcell.KeyUp:
			if t.status.creatingBuf {
				break
			}

			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeTab(curr - 1)
			}
		case tcell.KeyCtrlN:
			if !t.status.creatingBuf && !t.status.showingHelp {
				if t.tabs.Len() >= 35 {
					t.showError(ErrorMaxBufs)
					break
				}
				t.newbufPopup(app)
			}
		case tcell.KeyCtrlX:
			if t.status.creatingBuf || t.status.showingHelp {
				break
			}

			t.removeTab(t.active)
		case tcell.KeyCtrlR:
			app.Sync()
		}
		return event
	})
}

func New() (*TUI, *tview.Application) {
	areas, comps := setupLayout()
	t := &TUI{
		tabs: models.NewTable[string, *tab](0),
		comp: comps,
		area: areas,
		status: opts{
			showingUsers: false,
			showingBufs:  true,
			showingHelp:  false,
			creatingBuf:  false,
		},
	}
	app := tview.NewApplication().
		EnableMouse(true).
		SetRoot(t.area.main, true).
		SetFocus(t.area.main)

	setupKeybinds(t, app)
	setupHandlers(t, app)
	setupStyle(t)
	setupInput(t)

	t.newTab("System", true)
	t.active = "System"
	fmt.Fprint(t.comp.text, Logo[1:])
	t.SendMessage("System", Message{
		Sender:    "System",
		Content:   "Welcome to gochat!",
		Timestamp: time.Now(),
	})

	return t, app
}
