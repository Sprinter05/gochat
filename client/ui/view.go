package ui

import (
	"net"
	"sync"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// TODO: Order buffers when rendering server

/* TUI */

// Identifies conditions that may in any moment
// block another action from being performed, or
// gives instructions on how to render another element.
type state struct {
	showingUsers bool // Showing user list component
	showingBufs  bool // Showing buffer list component

	creatingBuf    bool // Creating a new buffer
	creatingServer bool // Creating a new server
	typingPassword bool // Inputting a password
	showingHelp    bool // Showing the help window

	lastDate time.Time // Last rendered date in the current buffer
}

// Identifies the main TUI with all its
// components and data.
type TUI struct {
	area areas              // Flex boxes with components
	comp components         // Actual tview components
	app  *tview.Application // App that runs

	status state           // Identifies rendering states
	data   cmds.StaticData // Identifies command data

	servers models.Table[string, Server] // Table storing servers
	focus   string                       // Currently active server
}

// Condition that prevents another operation from being performed
// depending on the state of the TUI.
func (s *state) blockCond() bool {
	return s.creatingBuf || s.creatingServer || s.showingHelp || s.typingPassword
}

/* POPUPS */

// Creates a basic popup with its corresponding blocking condition
// and puts the app's focus in said new popup. Returns the popup and
// a function to exit it.
func createPopup(t *TUI, cond *bool, title string) (*tview.InputField, func()) {
	*cond = true

	input := tview.NewInputField().
		SetPlaceholder(title).
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetPlaceholderStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow))
	input.SetBorder(false).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorderPadding(0, 0, 1, 0)

	t.area.bottom.ResizeItem(t.comp.input, 0, 0)
	t.area.bottom.AddItem(input, 2, 0, true)
	t.app.SetFocus(input)

	exit := func() {
		t.area.bottom.RemoveItem(input)
		t.area.bottom.ResizeItem(t.comp.input, inputSize, 0)
		t.app.SetFocus(t.comp.input)
		*cond = false
	}

	return input, exit
}

// Popup to create a new buffer that asks for the name of the
// buffer and then creates it, adds it to the TUI, and changes
// to it.
func newbufPopup(t *TUI) {
	input, exit := createPopup(t,
		&t.status.creatingBuf,
		"Enter buffer name...",
	)

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			exit()
			return
		}

		text := input.GetText()
		if text == "" {
			t.showError(ErrorNoText)
			return
		}

		t.addBuffer(text, false)

		exit()
	})
}

// Popup to create a new server that asks for the
// name of the server and the address, then creates
// it, adds it to the TUI and changes to it.
func newServerPopup(t *TUI) {
	sInput, sExit := createPopup(t,
		&t.status.creatingServer,
		"Enter server name...",
	)

	// Asks for name
	sInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			sExit()
			return
		}

		name := sInput.GetText()
		if name == "" {
			t.showError(ErrorNoText)
			return
		}

		sExit()

		pInput, pExit := createPopup(t,
			&t.status.creatingServer,
			"Enter server IP and port...",
		)

		// Asks for address
		pInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEscape {
				pExit()
				return
			}

			ip := pInput.GetText()
			if ip == "" {
				t.showError(ErrorNoText)
				return
			}

			addr, err := net.ResolveTCPAddr("tcp4", ip)
			if err != nil {
				t.showError(err)
				pExit()
				return
			}

			t.addServer(name, addr)
			t.addBuffer("Default", false)

			pExit()
		})
	})
}

// Popup that asks for a password. This popup is blocking, meaning
// that until the popup exits the function itself will not exit.
// Therefore this shouldn't run in the main thread as it will
// block all other components.
func newLoginPopup(t *TUI) (pswd string, err error) {
	cond := sync.NewCond(new(sync.Mutex))
	cond.L.Lock()
	defer cond.L.Unlock()

	input, exit := createPopup(t,
		&t.status.typingPassword,
		"Enter password...",
	)

	input.SetMaskCharacter('*')
	input.SetDoneFunc(func(key tcell.Key) {
		cond.L.Lock()
		defer cond.L.Unlock()

		if key == tcell.KeyEscape {
			err = ErrorNoText
			exit()
			cond.Signal()
			return
		}

		text := input.GetText()
		if text == "" {
			t.showError(ErrorNoText)
			return
		}

		pswd = text
		err = nil
		cond.Signal()
		exit()
	})

	cond.Wait()
	return pswd, err
}
