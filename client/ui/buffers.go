package ui

import (
	"fmt"
	"time"

	cmds "github.com/Sprinter05/gochat/client/commands"
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
)

/* STRUCTS */

// Specifies a buffer inside of a server that can
// be shown or hidden.
type tab struct {
	index    int    // Identifies the order of the buffer
	name     string // Identifies the name
	creation int    // Identifies the internal buffer list order

	messages models.Slice[Message] // Messages stored in the buffer

	connected bool // Whether its asocciated to a server endpoint or not
	system    bool // Whether it was created by the system
}

// Identifies all the buffers that conform a server. All asocciated
// functions are independent of the TUI rendering.
type Buffers struct {
	tabs    models.Table[string, *tab] // Table storing the buffers
	current string                     // Currently open buffer
	open    int                        // How many buffers are open
	indexes []int                      // Free indexes left by hidden buffers
}

/* HELPER FUNCTIONS */

// Returns the rune asocciated to a buffer's index
// Can be 1-9 or a-z.
func ascii(num int) int32 {
	offset := asciiNumbers + num
	if num >= 10 {
		offset = asciiLowercase + (num - 10)
	}

	return int32(offset)
}

// Returns the currently active tab
// in the current server. Only returns
// the name and not the actual data.
func (t *TUI) Buffer() string {
	return t.Active().Buffers().current
}

/* BUFFERS */

// Creates a new buffer (initially hidden).
func (b *Buffers) New(name string, system bool) error {
	_, ok := b.tabs.Get(name)
	if ok {
		return ErrorExists
	}

	tab := &tab{
		index:     -1,
		name:      name,
		creation:  b.tabs.Len() + 1,
		messages:  models.NewSlice[Message](0),
		system:    system,
		connected: false,
	}

	b.tabs.Add(name, tab)
	return nil
}

// Gets all the buffer names
func (b *Buffers) GetAll() []string {
	list := make([]string, 0, b.tabs.Len())

	copy := b.tabs.GetAll()
	for _, v := range copy {
		list = append(list, v.name)
	}

	return list
}

// Assigns the buffer as online and returns whether it failed or not
func (b *Buffers) Current() *tab {
	t, ok := b.tabs.Get(b.current)
	if !ok {
		return nil
	}

	return t
}

// Turns all tabs to offline
func (b *Buffers) Offline() {
	list := b.tabs.GetAll()

	for _, v := range list {
		v.connected = false
	}
}

// Assigns an index to a hidden buffer (unless it was not hidden)
// and returns the index and asocciated rune. If any index
// was left by another buffer it will be grabbed first.
func (b *Buffers) Show(name string) (int, rune) {
	t, ok := b.tabs.Get(name)
	if !ok {
		return -1, -1
	}

	if t.index != -1 {
		return -1, -1
	}

	b.open += 1
	t.index = b.open

	// We check if theres an available index
	// left by another buffer
	l := len(b.indexes)
	if l > 0 {
		t.index = b.indexes[0]    // FIFO
		b.indexes = b.indexes[1:] // Remove
	}

	return b.open - 1, ascii(t.index)
}

// Hides a buffer by removing its index and putting it
// for use by any buffer created in the future.
func (b *Buffers) Hide(name string) error {
	t, ok := b.tabs.Get(name)
	if !ok {
		return ErrorNotFound
	}

	if t.system {
		return ErrorSystemBuf
	}

	// Already hidden
	if t.index == -1 {
		return nil
	}

	b.open -= 1
	b.indexes = append(b.indexes, t.index)
	t.index = -1
	return nil
}

// Deletes all information about a buffer. It hides
// the buffer first to recover its index.
func (b *Buffers) Remove(name string) error {
	t, ok := b.tabs.Get(name)
	if !ok {
		return ErrorNotFound
	}

	if t.system {
		return ErrorSystemBuf
	}

	b.Hide(name)
	b.tabs.Remove(name)
	return nil
}

/* RENDERING */

// Adds a buffer to the currently active server,
// shows it, and changes to the newly created buffer.
func (t *TUI) addBuffer(name string, system bool) {
	s := t.Active()
	if s.Buffers().open >= int(maxBuffers) {
		t.showError(ErrorMaxBufs)
		return
	}

	// If we are in a remote server we have to check
	// that we are online and that we are requesting
	// a user that is not the logged in user
	data, online := s.Online()
	if data != nil && name != defaultBuffer {
		if !online || !data.IsLoggedIn() {
			t.showError(ErrorNotLoggedIn)
			return
		}

		if name == data.LocalUser.User.Username {
			t.showError(ErrorMessageSelf)
			return
		}
	}

	err := s.Buffers().New(name, system)
	_, ok := t.findBuffer(name)
	if err != nil && ok {
		t.showError(err)
		return
	}

	i, r := s.Buffers().Show(name)
	if i == -1 {
		return
	}
	t.comp.buffers.AddItem(name, "", r, nil)

	// We try to request the user first
	empty := func(string, cmds.OutputType) {}
	reqErr := t.requestUser(s, name, empty)
	if reqErr != nil {
		print := t.systemMessage("request")
		print(reqErr.Error(), cmds.ERROR)

		t.hideBuffer(name)
		t.removeBuffer(name)
		return
	}

	t.changeBuffer(i)
}

// Changes the TUI component according to the internal
// index of the list and then renders the buffer.
func (t *TUI) changeBuffer(i int) {
	if i < 0 || i >= t.comp.buffers.GetItemCount() {
		return
	}

	t.comp.buffers.SetCurrentItem(i)
	text, _ := t.comp.buffers.GetItemText(i)
	t.renderBuffer(text)
}

// Finds the internal index of a buffer by its name
// in the TUI component. Returns whether it was found
// or not as well.
func (t *TUI) findBuffer(name string) (int, bool) {
	l := t.comp.buffers.FindItems(name, "", true, false)

	if len(l) != 0 {
		return l[0], true
	}

	return -1, false
}

// Hides a buffer from the TUI component and changes to
// the previous buffer unless the position was at the top,
// in which case it changes to the next buffer. This does
// not delete the buffer data. It will not change position
// if that buffer wasnt the targeted one.
func (t *TUI) hideBuffer(name string) {
	err := t.Active().Buffers().Hide(name)
	if err != nil {
		t.showError(err)
		return
	}

	// If the hidden buffer is the one we are targeting
	// we change to another buffer
	if t.Buffer() == name {
		count := t.comp.buffers.GetItemCount()
		if count == 1 {
			// All buffers have been deleted
			t.comp.text.Clear()
			t.Active().Buffers().current = ""
		} else {
			curr := t.comp.buffers.GetCurrentItem()
			if curr == 0 {
				t.changeBuffer(curr + 1)
			} else {
				t.changeBuffer(curr - 1)
			}
		}
	}

	i, ok := t.findBuffer(name)
	if ok {
		t.comp.buffers.RemoveItem(i)
	}
}

// Deletes a buffer instead of hiding it
// This assumes the buffer has already been deleted from the TUI.
func (t *TUI) removeBuffer(name string) error {
	err := t.Active().Buffers().Remove(name)
	if err != nil {
		return err
	}

	return nil
}

// Shows all messages in a buffer and changes the color of the
// TUI component for system buffers. It assumes the buffer has
// already been changed in the TUI component. It also sets the
// variable controlling the currently rendered buffer.
func (t *TUI) renderBuffer(buf string) {
	s := t.Active()
	b, ok := s.Buffers().tabs.Get(buf)
	if !ok {
		return
	}

	s.Buffers().current = buf

	if b.system {
		t.comp.buffers.SetSelectedTextColor(tcell.ColorPlum)
	} else {
		t.comp.buffers.SetSelectedTextColor(tcell.ColorPurple)
	}

	if t.status.showingHelp {
		return
	}

	// Render text
	t.status.lastDate = time.Now()
	t.comp.text.Clear()
	msgs := s.Messages(buf)

	// Render notifications
	l := len(msgs)
	pending := s.Notifications().Query(buf)
	unread := l - int(pending)

	for i, v := range msgs {
		t.renderMsg(v)

		if (unread-1) == i && pending > 0 {
			fmt.Fprintf(
				t.comp.text,
				"--- [orange]UNREAD[-] ---\n",
			)
		}
	}
	t.updateNotifications()
}
