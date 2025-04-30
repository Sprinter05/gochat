package ui

import (
	"time"

	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type state struct {
	showingUsers bool
	showingBufs  bool

	creatingBuf bool
	showingHelp bool
	lastDate    time.Time
}

type TUI struct {
	area areas
	comp components

	status state

	servers models.Table[string, Server]
	active  string
}

func newbufPopup(t *TUI, app *tview.Application) {
	t.status.creatingBuf = true

	input := tview.NewInputField().
		SetPlaceholder("Enter buffer name...").
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetPlaceholderStyle(tcell.StyleDefault.
			Background(tcell.ColorDefault).
			Foreground(tcell.ColorYellow))
	input.SetBorder(false).
		SetBackgroundColor(tcell.ColorDefault).
		SetBorderPadding(0, 0, 1, 0)

	t.area.chat.ResizeItem(t.comp.input, 0, 0)
	t.area.chat.AddItem(input, 2, 0, true)
	app.SetFocus(input)

	exit := func() {
		t.area.chat.RemoveItem(input)
		t.area.chat.ResizeItem(t.comp.input, inputSize, 0)
		app.SetFocus(t.comp.input)
		t.status.creatingBuf = false
	}

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

		s := t.Active()
		if s.Buffers().tabs.Len() > int(maxBuffers) {
			t.showError(ErrorMaxBufs)
			return
		}

		t.addBuffer(text, false)

		exit()
	})
}

// Adds and changes to new buffer on the list
func (t *TUI) addBuffer(name string, system bool) {
	s := t.Active()
	i, r, err := s.Buffers().New(name, system)
	if err != nil {
		t.showError(err)
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

// Removes and changes buffer on the list
func (t *TUI) removeBuffer(name string) {
	err := t.Active().Buffers().Remove(name)
	if err != nil {
		t.showError(err)
		return
	}

	count := t.comp.buffers.GetItemCount()
	if count == 1 {
		t.comp.text.Clear()
	} else {
		curr := t.comp.buffers.GetCurrentItem()
		if curr == 0 {
			t.changeBuffer(curr + 1)
		} else {
			t.changeBuffer(curr - 1)
		}
	}

	l := t.comp.buffers.FindItems(name, "", true, false)
	for _, v := range l {
		t.comp.buffers.RemoveItem(v)
	}

}
