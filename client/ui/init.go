package ui

import (
	"errors"
	"fmt"
	"net"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
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

const Help string = `
[-::u]Keybinds Manual:[-::-]

[yellow::b]Ctrl-Alt-H/Ctrl-Shift-H[-::-]: Show/Hide help window

[yellow::b]Ctrl-Q[-::-]: Exit program

[yellow::b]Ctrl-T[-::-]: Focus chat/input window
	- In the [-::b]chat window[-::-] use [green]Up/Down[-::-] to move
	- In the [-::b]input window[-::-] use [green]Alt-Enter/Shift-Enter[-::-] to add a newline


[yellow::b]Ctrl-K + Ctrl-N[-::-]: Create a new buffer
	- [green]Esc[-::-] to cancel
	- [green]Enter[-::-] to confirm

[yellow::b]Ctrl-K + Ctrl-X[-::-]: Hide currently focused buffer
	- It can be shown again by creating a buffer with the same name

[yellow::b]Ctrl-K[-::-] + [green::b]1-z[-::-]: Jump to specific buffer
	- Press [green]Esc[-::-] to cancel the jump

[yellow::b]Ctrl-S + Ctrl-N[-::-]: Create a new server
	- [green]Esc[-::-] to cancel
	- [green]Enter[-::-] to confirm the different steps
	
[yellow::b]Ctrl-S + Ctrl-X[-::-]: Delete currently focused server
	
[yellow::b]Ctrl-S[-::-] + [green::b]1-9[-::-]: Jump to specific server
	- Press [green]Esc[-::-] to cancel the jump
	
[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Alt-Up/Down[-::-]: Go to next/previous buffer

[yellow::b]Ctrl-B[-::-]: Show/Hide buffer list

[yellow::b]Ctrl-U[-::-]: Show/Hide user list

[yellow::b]Ctrl-R[-::-]: Redraw screen

[-::u]Commands Manual:[-::-]

[yellow::b]/list[-::-]: Displays a list of all buffers in the current server
	- Those that have been hidden will also be displayed
`

const (
	selfSender     string = "You"
	systemBuffer   string = "System"
	localServer    string = "Local"
	inputSize      int    = 4
	errorMessage   uint   = 3    // seconds
	asciiNumbers   int    = 0x30 // Start of ASCII for number 1
	asciiLowercase int    = 0x61 // Start of ASCII for lowercase a
	maxBuffers     uint   = 35
	maxServers     uint   = 9
	maxMsgs        uint   = 5
	maxCmds        uint   = 10
)

var (
	ErrorSystemBuf     = errors.New("performing action on system buffer")
	ErrorLocalServer   = errors.New("performing action on local server")
	ErrorNoText        = errors.New("no text has been given")
	ErrorExists        = errors.New("item already exists")
	ErrorNotFound      = errors.New("item does not exist")
	ErrorMaxBufs       = errors.New("maximum amount of buffers reached")
	ErrorMaxServers    = errors.New("maximum amount of servers reached")
	ErrorNoBuffers     = errors.New("no buffers in server")
	ErrorEmptyCmd      = errors.New("empty command given")
	ErrorInvalidCmd    = errors.New("invalid command given")
	ErrorAlreadyOnline = errors.New("connection is already established")
	ErrorLocal         = errors.New("cannot connect on local server")
	ErrorOffline       = errors.New("connection to the server is not established")
	ErrorArguments     = errors.New("invalid number of arguments")
)

type areas struct {
	main   *tview.Flex
	bottom *tview.Flex
	left   *tview.Flex
}

type components struct {
	buffers *tview.List
	servers *tview.List

	text   *tview.TextView
	errors *tview.TextView
	input  *tview.TextArea

	users *tview.List
}

func setupLayout() (areas, components) {
	comps := components{
		buffers: tview.NewList(),
		servers: tview.NewList(),
		text:    tview.NewTextView(),
		errors:  tview.NewTextView(),
		input:   tview.NewTextArea(),
		users:   tview.NewList(),
	}

	bottom := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.text, 0, 30, false).
		AddItem(comps.errors, 0, 0, false).
		AddItem(comps.input, inputSize, 0, true)
	bottom.SetBackgroundColor(tcell.ColorDefault)

	left := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(comps.buffers, 0, 3, false).
		AddItem(comps.servers, 0, 1, false)
	left.SetBackgroundColor(tcell.ColorDefault)

	main := tview.NewFlex().
		AddItem(left, 0, 2, false).
		AddItem(bottom, 0, 6, true).
		AddItem(comps.users, 0, 0, false)
	main.SetBackgroundColor(tcell.ColorDefault)

	areas := areas{
		main:   main,
		bottom: bottom,
		left:   left,
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
		SetMainTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetShortcutStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow)).
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

	t.comp.servers.
		SetMainTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetSecondaryTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorDarkGray)).
		SetShortcutStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow)).
		ShowSecondaryText(true).
		SetSelectedStyle(tcell.StyleDefault.Underline(true)).
		SetSelectedTextColor(tcell.ColorPurple).
		SetTitle("Servers").
		SetBorder(true).
		SetBackgroundColor(tcell.ColorDefault)

	t.comp.input.
		SetLabel(" > ").
		SetTextStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault)).
		SetPlaceholderStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorGreen)).
		SetPlaceholder("Write here...").
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(true)

	t.comp.errors.
		SetDynamicColors(true).
		SetWrap(true).
		SetWordWrap(true).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorder(false)
}

func setupHandlers(t *TUI, app *tview.Application) {
	t.comp.buffers.SetDoneFunc(func() {
		app.SetFocus(t.comp.input)
	})

	t.comp.servers.SetDoneFunc(func() {
		app.SetFocus(t.comp.input)
	})

	t.comp.buffers.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		t.renderBuffer(s1)
		app.SetFocus(t.comp.input)
	})

	t.comp.servers.SetSelectedFunc(func(i int, s1, s2 string, r rune) {
		t.renderServer(s1)
		app.SetFocus(t.comp.input)
	})

	t.comp.buffers.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			if !t.status.blockCond() {
				newbufPopup(t, app)
			}
		case tcell.KeyCtrlX:
			if !t.status.blockCond() {
				t.removeBuffer(t.Buffer())
				app.SetFocus(t.comp.input)
			}
		}
		return event
	})

	t.comp.servers.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			if !t.status.blockCond() {
				newServerPopup(t, app)
			}
		case tcell.KeyCtrlX:
			if !t.status.blockCond() {
				t.removeServer(t.active)
				app.SetFocus(t.comp.input)
			}
		}
		return event
	})

	t.comp.text.SetChangedFunc(func() {
		app.Draw()
	})

	t.comp.errors.SetChangedFunc(func() {
		app.Draw()
	})
}

func setupInput(t *TUI) {
	t.comp.input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			if event.Modifiers()&tcell.ModShift == tcell.ModShift ||
				event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				return event
			}

			text := t.comp.input.GetText()
			if text == "" {
				return nil
			}

			if t.Buffer() == "" {
				t.showError(ErrorNoBuffers)
				return nil
			}

			if text[0] == '/' {
				t.parseCommand(text[1:])
				t.comp.input.SetText("", false)
				return nil
			}

			s := t.Active()
			t.SendMessage(Message{
				Sender:    selfSender,
				Buffer:    t.Buffer(),
				Content:   text,
				Timestamp: time.Now(),
				Source:    s.Source(),
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
				t.area.main.ResizeItem(t.area.left, 0, 0)
				t.status.showingBufs = false
			} else {
				t.area.main.ResizeItem(t.area.left, 0, 2)
				t.status.showingBufs = true
			}
		case tcell.KeyCtrlT:
			if t.status.blockCond() {
				break
			}

			if !t.comp.text.HasFocus() {
				app.SetFocus(t.comp.text)
				return nil
			} else {
				app.SetFocus(t.comp.input)
				return nil
			}
		case tcell.KeyCtrlS:
			if t.status.blockCond() {
				break
			}

			if !t.comp.servers.HasFocus() {
				app.SetFocus(t.comp.servers)
				return nil
			}
		case tcell.KeyCtrlH:
			if event.Modifiers()&tcell.ModShift == tcell.ModShift ||
				event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				app.SetFocus(t.comp.text)
				t.toggleHelp()
				if !t.status.showingHelp {
					app.SetFocus(t.comp.input)
				}
			}
		case tcell.KeyCtrlK:
			if t.status.blockCond() {
				break
			}

			if !t.comp.buffers.HasFocus() {
				app.SetFocus(t.comp.buffers)
				return nil
			}
		case tcell.KeyDown:
			if t.status.blockCond() {
				break
			}

			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeBuffer(curr + 1)
			}
		case tcell.KeyUp:
			if t.status.blockCond() {
				break
			}

			if event.Modifiers()&tcell.ModAlt == tcell.ModAlt {
				curr := t.comp.buffers.GetCurrentItem()
				t.changeBuffer(curr - 1)
			}
		case tcell.KeyCtrlR:
			app.Sync()
		}
		return event
	})
}

func New(static cmds.StaticData) (*TUI, *tview.Application) {
	areas, comps := setupLayout()
	t := &TUI{
		servers: models.NewTable[string, Server](0),
		comp:    comps,
		area:    areas,
		status: state{
			showingUsers:   false,
			showingBufs:    true,
			showingHelp:    false,
			creatingBuf:    false,
			creatingServer: false,
			lastDate:       time.Now(),
		},
		data: static,
	}
	app := tview.NewApplication().
		EnableMouse(true).
		SetRoot(t.area.main, true).
		SetFocus(t.area.main)

	setupKeybinds(t, app)
	setupHandlers(t, app)
	setupStyle(t)
	setupInput(t)

	// system server
	t.servers.Add(localServer, &LocalServer{
		name: localServer,
		bufs: Buffers{
			tabs: models.NewTable[string, *tab](maxBuffers),
		},
	})
	t.active = localServer
	t.addBuffer(systemBuffer, true)
	l := t.servers.Len()
	t.comp.servers.AddItem(localServer, "System Server", ascii(l), nil)

	print := t.systemMessage()
	print("Welcome to gochat!")
	print("Restoring previous session...")
	t.restoreSession()

	return t, app
}

func (t *TUI) restoreSession() {
	// Restore servers
	list := db.GetAllServers(t.data.DB)
	for _, v := range list {
		str := fmt.Sprintf("%s:%d", v.Address, v.Port)
		addr, _ := net.ResolveTCPAddr("tcp4", str)
		t.addServer(v.Name, addr)
	}
}
