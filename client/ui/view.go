package ui

import (
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

/* TUI */

type userlistUser struct {
	name  string
	perms uint
}

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

	deletingServer bool // Currently choosing to delete server
	deletingBuffer bool // Currently choosing to delete buffer

	userlist models.Slice[userlistUser] // Used for displaying users in the user bar

	lastDate time.Time // Last rendered date in the current buffer
	lastMsg  time.Time // last message sent
}

// Identifies the main TUI with all its
// components and data.
type TUI struct {
	area areas              // Flex boxes with components
	comp components         // Actual tview components
	app  *tview.Application // App that runs

	status state           // Identifies rendering states
	data   cmds.StaticData // Identifies command data

	history models.Slice[string] // Stores previously ran commands
	next    uint                 // Last history

	servers models.Table[string, Server] // Table storing servers
	focus   string                       // Currently active server
}

// Condition that prevents another operation from being performed
// depending on the state of the TUI.
func (s *state) blockCond() bool {
	return s.creatingBuf ||
		s.creatingServer ||
		s.showingHelp ||
		s.typingPassword ||
		s.deletingServer ||
		s.deletingBuffer
}

/* USERLIST */

func (s *state) userlistRender() string {
	var list strings.Builder

	if s.userlist.Len() == 0 {
		return ""
	}

	copy := s.userlist.Copy(0)
	slices.SortFunc(copy, func(a, b userlistUser) int {
		if a.perms < b.perms {
			return 1
		} else if a.perms > b.perms {
			return -1
		}

		return 0
	})

	for _, v := range copy {
		str := fmt.Sprintf(
			"[[purple::i]%d[-::-]] %s\n",
			v.perms, v.name,
		)
		list.WriteString(str)
	}

	ret := list.String()
	l := len(ret)
	return ret[:l-1]
}

func (s *state) userlistAdd(name string, perms uint) {
	s.userlist.Add(userlistUser{
		name:  name,
		perms: perms,
	})
}

func (s *state) userlistRemove(name string) {
	val, ok := s.userlist.Find(func(uu userlistUser) bool {
		return uu.name == name
	})

	if ok {
		s.userlist.Remove(val)
	}
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
	t.app.EnableMouse(false)

	exit := func() {
		t.area.bottom.RemoveItem(input)
		t.area.bottom.ResizeItem(t.comp.input, inputSize, 0)
		t.app.SetFocus(t.comp.input)
		t.app.EnableMouse(true)
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

		name := strings.ToLower(text)

		t.addBuffer(name, false)

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

			addr, err := net.ResolveTCPAddr("tcp", ip)
			if err != nil {
				t.showError(err)
				pExit()
				return
			}

			// We enable TLS by default
			ret := t.addServer(name, addr, true)
			if ret != nil {
				t.showError(ret)
			} else {
				t.addBuffer(defaultBuffer, true)
				welcomeMessage(t)
			}

			pExit()
		})
	})
}

// Popup that asks for a password. This popup is blocking, meaning
// that until the popup exits the function itself will not exit.
// Therefore this shouldn't run in the main thread as it will
// block all other components.
func newLoginPopup(t *TUI, text string) (pswd string, err error) {
	cond := sync.NewCond(new(sync.Mutex))
	cond.L.Lock()
	defer cond.L.Unlock()

	input, exit := createPopup(t,
		&t.status.typingPassword,
		text,
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

/* CONFIRMATION WINDOWS */

// Creates a basic confirmation window with "Yes" or "No" choices for a
// given operation. Returns the window and the function to exit it.
func createConfirmWindow(t *TUI, cond *bool, title string) (*tview.Modal, func()) {
	*cond = true

	window := tview.NewModal().
		SetText(title).
		AddButtons([]string{"Yes", "No"})
	window.SetBackgroundColor(tcell.ColorDefault).
		SetBorderStyle(tcell.StyleDefault).
		SetBorder(true)

	t.area.main.AddItem(window, 0, 0, true)
	t.app.SetFocus(window)
	t.app.EnableMouse(false)

	exit := func() {
		t.area.main.RemoveItem(window)
		t.app.SetFocus(t.comp.input)
		t.app.EnableMouse(true)
		*cond = false
	}

	return window, exit
}

// Confirmation window to delete a server from the TUI
// and also from the database.
func deleteServWindow(t *TUI) {
	window, exit := createConfirmWindow(t,
		&t.status.deletingServer,
		"Do you want to permanently\ndelete this server?",
	)

	window.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Yes" {
			t.removeServer(t.Active())
			t.hideServer(t.focus)
		}

		exit()
	})
}

// Confirmation window to delete a buffer from the TUI
// and also all related messages from the database.
func deleteBufWindow(t *TUI) {
	window, exit := createConfirmWindow(t,
		&t.status.deletingBuffer,
		"Do you want to permanently\ndelete this buffer?",
	)

	window.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		if buttonLabel == "Yes" {
			buf := t.Buffer()
			t.hideBuffer(buf)

			err := t.removeBuffer(buf)
			if err != nil {
				t.showError(err)
			}
		}

		exit()
	})
}

/* BARS */

func toggleBufList(t *TUI) {
	if t.status.showingBufs {
		t.area.main.ResizeItem(t.area.left, 0, 0)
		t.status.showingBufs = false
	} else {
		t.area.main.ResizeItem(t.area.left, 0, 2)
		t.status.showingBufs = true
	}
}

func toggleUserlist(t *TUI) {
	if t.status.showingUsers {
		t.area.main.ResizeItem(t.comp.users, 0, 0)
		t.status.showingUsers = false
	} else {
		t.area.main.ResizeItem(t.comp.users, 0, 1)
		t.status.showingUsers = true
	}
}

func updateOnlineUsers(t *TUI, s Server, output cmds.OutputFunc) {
	data, ok := s.Online()
	t.status.userlist.Clear()

	if data == nil || !ok {
		t.comp.users.SetText(defaultUserlist)
		return
	}

	cmd := cmds.Command{
		Output: output,
		Static: &t.data,
		Data:   data,
	}

	ctx, cancel := timeout(s, data)
	defer data.Waitlist.Cancel(cancel)
	reply, err := cmds.Usrs(ctx, cmd, cmds.ONLINEPERMS)

	if err != nil {
		output(err.Error(), cmds.ERROR)
		return
	}

	for _, v := range reply {
		name, perms, _ := strings.Cut(string(v), " ")
		val, err := strconv.Atoi(perms)
		if err != nil {
			val = 0
		}
		t.status.userlistAdd(name, uint(val))
	}

	t.comp.users.SetText(t.status.userlistRender())
}
