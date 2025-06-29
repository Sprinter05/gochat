package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/client/db"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gorm.io/gorm"
)

/* TUI */

// Struct representing a user shown in the userlist
type userlistUser struct {
	name  string // Name of the user
	perms uint   // Permission level of the user
}

// Identifies conditions that may in any moment
// block another action from being performed, or
// gives instructions on how to render another element.
type state struct {
	showingUsers bool // Showing user list component
	showingBufs  bool // Showing buffer list component

	creatingBuf        bool // Creating a new buffer
	creatingServer     bool // Creating a new server
	typingPassword     bool // Inputting a password
	showingHelp        bool // Showing the help window
	showingQuickswitch bool // Showing the quickswitch input

	deletingServer bool // Currently choosing to delete server
	deletingBuffer bool // Currently choosing to delete buffer

	userlist      models.Slice[userlistUser] // Used for displaying users in the user bar
	serverIndexes []int                      // Used to track deleted elements

	lastDate time.Time // Last rendered date in the current buffer
	lastMsg  time.Time // last message sent
}

// Used to change size of a specific component
type ComponentSize struct {
	Size     uint // Specifies the size of the component
	Relative bool // Specifies whether its relative to the other components
}

// Used to modify the sizes of the components
// in the TUI for its configuration.
// Must be exported for external modification
type Parameters struct {
	Buflist  ComponentSize // Size of left bar
	Userlist ComponentSize // Size of right bar
	Verbose  bool          // Whether to print verbose or not
}

// Identifies the main TUI with all its
// components and data.
type TUI struct {
	area areas              // Flex boxes with components
	comp components         // Actual tview components
	app  *tview.Application // App that runs

	params Parameters // Size of the different components
	status state      // Identifies rendering states
	db     *gorm.DB   // Identifies the database to be used

	history models.Slice[string] // Stores previously ran commands
	next    uint                 // Last history

	servers models.Table[string, Server] // Table storing servers
	focus   string                       // Currently active server
}

// Returns a static data for use on a command
func (t *TUI) static() *cmds.StaticData {
	return &cmds.StaticData{
		DB:      t.db,
		Verbose: t.params.Verbose,
	}
}

// Condition that prevents another operation from being performed
// depending on the state of the TUI.
func (s *state) blockCond() bool {
	return s.creatingBuf ||
		s.creatingServer ||
		s.showingHelp ||
		s.typingPassword ||
		s.deletingServer ||
		s.deletingBuffer ||
		s.showingQuickswitch
}

/* USERLIST */

// Renders the userlist of whatever is saved as the current state
func (s *state) userlistRender() string {
	var list strings.Builder

	if s.userlist.Len() == 0 {
		return ""
	}

	// Sort by perms
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

// Change the permissing level of a user in the userlist
func (s *state) userlistChange(name string, perms uint) {
	val, ok := s.userlist.Find(func(uu userlistUser) bool {
		return uu.name == name
	})

	if ok {
		// If it already existed we remove it
		// to add with new perms
		s.userlist.Remove(val)
	}

	s.userlist.Add(userlistUser{
		name:  name,
		perms: perms,
	})
}

// Remove a user from the userlist
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

		exists, err := db.ServerExistsByName(t.db, name)
		if err != nil {
			t.showError(err)
			return
		}

		if exists {
			err := t.showServer(name)
			if err != nil {
				t.showError(err)
			} else {
				t.addBuffer(defaultBuffer, true)
				welcomeMessage(t)
			}

			sExit()
			return
		}

		sExit()

		pInput, pExit := createPopup(t,
			&t.status.creatingServer,
			"Enter server address and port as 'address:port':",
		)

		// Asks for address and port
		pInput.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEscape {
				pExit()
				return
			}

			text := pInput.GetText()
			if text == "" {
				t.showError(ErrorNoText)
				return
			}

			addr, num, ok := strings.Cut(text, ":")
			if !ok {
				t.showError(ErrorInvalidAddress)
				return
			}

			port, err := strconv.ParseUint(num, 10, 16)
			if err != nil || port == 0 {
				t.showError(ErrorInvalidAddress)
				return
			}

			// We enable TLS by default
			ret := t.addServer(name, addr, uint16(port), true)
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
func newPasswordPopup(t *TUI, text string) (pswd string, err error) {
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

// Creates a popup to quickly switch to any buffer
func newQuickSwitchPopup(t *TUI) {
	input, exit := createPopup(t, &t.status.showingQuickswitch, "Go to...")
	input.SetAutocompleteFunc(func(currentText string) []string {
		if len(currentText) == 0 {
			return nil
		}

		list := t.Active().Buffers().GetAll()
		ret := make([]string, 0, len(list))
		for _, v := range list {
			// Autocomplete entries on text change
			target := strings.ToLower(v)
			curr := strings.ToLower(currentText)

			if strings.HasPrefix(target, curr) {
				ret = append(ret, v)
			}
		}

		if len(ret) == 0 {
			return nil
		}

		return ret
	})

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

		i, ok := t.findBuffer(text)
		if !ok {
			t.showError(ErrorNoText)
			return
		}

		t.changeBuffer(i)
		exit()
	})
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
		"Do you want to permanently\ndelete this server?\nAll information about this server will be lost!",
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

// Renders the bufferlist depending on the size and mode
func renderBuflist(t *TUI) {
	if t.status.showingBufs {
		if t.params.Buflist.Relative {
			t.area.main.ResizeItem(
				t.area.left,
				0,
				int(t.params.Buflist.Size),
			)
		} else {
			t.area.main.ResizeItem(
				t.area.left,
				int(t.params.Buflist.Size),
				0,
			)
		}
		return
	}

	t.area.main.ResizeItem(t.area.left, 0, 0)
}

// Renders the userlist depending on the size and mode
func renderUserlist(t *TUI) {
	if t.status.showingUsers {
		if t.params.Userlist.Relative {
			t.area.main.ResizeItem(
				t.comp.users,
				0,
				int(t.params.Userlist.Size),
			)
		} else {
			t.area.main.ResizeItem(
				t.comp.users,
				int(t.params.Userlist.Size),
				0,
			)
		}
		return
	}

	t.area.main.ResizeItem(t.comp.users, 0, 0)
}

// Enables or disables the buffer list
func toggleBufList(t *TUI) {
	t.status.showingBufs = !t.status.showingBufs
	renderBuflist(t)
}

// Enables or disables the user list
func toggleUserlist(t *TUI) {
	t.status.showingUsers = !t.status.showingUsers
	renderUserlist(t)
}

/* RENDER FUNCTIONS */

// Update the text of all servers showing up on the list
func updateServers(t *TUI) {
	list := t.servers.Indexes()

	for _, v := range list {
		s, _ := t.servers.Get(v)

		// Update the data
		s.Update()

		// Ignore local servers
		data, _ := s.Online()
		if data == nil {
			continue
		}

		// Remap it with the right index
		name := s.Name()
		if name != v {
			t.servers.Remove(v)
			t.servers.Add(name, s)

			// Update the focus
			if t.focus == v {
				t.focus = name
			}
		}

		// Get the TUI object
		i, ok := t.findServer(v)
		if !ok {
			continue
		}

		// Modify the name in the TUI
		t.comp.servers.SetItemText(
			i, name,
			tlsText(
				s.Source(),
				data.Server.TLS,
			),
		)
	}
}

// Updates the list of online users when connected to a server
func updateOnlineUsers(t *TUI, s Server, output cmds.OutputFunc) {
	data, ok := s.Online()
	t.status.userlist.Clear()

	if data == nil || !ok {
		t.comp.users.SetText(defaultUserlist)
		return
	}

	cmd := cmds.Command{
		Output: output,
		Static: t.static(),
		Data:   data,
	}

	ctx, cancel := timeout(s, data)
	defer data.Waitlist.Cancel(cancel)
	reply, err := cmds.USRS(ctx, cmd, cmds.ONLINEPERMS)

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
		t.status.userlistChange(name, uint(val))
	}

	t.comp.users.SetText(t.status.userlistRender())
}
