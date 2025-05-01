package ui

import (
	"net"
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type state struct {
	showingUsers bool
	showingBufs  bool

	creatingBuf    bool
	creatingServer bool
	showingHelp    bool

	lastDate time.Time
}

type TUI struct {
	area areas
	comp components

	status state

	servers models.Table[string, Server]
	active  string

	reqs chan Command
}

func (s *state) blockCond() bool {
	return s.creatingBuf || s.creatingServer || s.showingHelp
}

func createPopup(t *TUI, app *tview.Application, cond *bool, title string) (*tview.InputField, func()) {
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
	app.SetFocus(input)

	exit := func() {
		t.area.bottom.RemoveItem(input)
		t.area.bottom.ResizeItem(t.comp.input, inputSize, 0)
		app.SetFocus(t.comp.input)
		*cond = false
	}

	return input, exit
}

func newbufPopup(t *TUI, app *tview.Application) {
	input, exit := createPopup(t, app,
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

func newServerPopup(t *TUI, app *tview.Application) {
	sInput, sExit := createPopup(t, app,
		&t.status.creatingServer,
		"Enter server name...",
	)

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

		pInput, pExit := createPopup(t, app,
			&t.status.creatingServer,
			"Enter server IP and port...",
		)

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
			pExit()
		})
	})
}

// Adds and changes to new buffer on the list
func (t *TUI) addBuffer(name string, system bool) {
	s := t.Active()
	if s.Buffers().open >= int(maxBuffers) {
		t.showError(ErrorMaxBufs)
		return
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
	t.changeBuffer(i)
}

// Changes to buffers on the list
func (t *TUI) changeBuffer(i int) {
	if i < 0 || i >= t.comp.buffers.GetItemCount() {
		return
	}

	t.comp.buffers.SetCurrentItem(i)
	text, _ := t.comp.buffers.GetItemText(i)
	t.renderBuffer(text)
}

func (t *TUI) findBuffer(name string) (int, bool) {
	l := t.comp.buffers.FindItems(name, "", true, false)

	if len(l) != 0 {
		return l[0], true
	}

	return -1, false
}

// Removes and changes buffer on the list
func (t *TUI) removeBuffer(name string) {
	err := t.Active().Buffers().Hide(name)
	if err != nil {
		t.showError(err)
		return
	}

	count := t.comp.buffers.GetItemCount()
	if count == 1 {
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

	i, ok := t.findBuffer(name)
	if ok {
		t.comp.buffers.RemoveItem(i)
	}
}
