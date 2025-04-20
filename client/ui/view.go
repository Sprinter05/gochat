package ui

import (
	"github.com/Sprinter05/gochat/internal/models"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type tab struct {
	index    int
	messages models.Slice[Message]
	name     string
	system   bool
}

type opts struct {
	showingUsers bool
	showingBufs  bool
	creatingBuf  bool
	showingHelp  bool
	freeIndexes  []int
}

type TUI struct {
	area   areas
	comp   components
	tabs   models.Table[string, *tab]
	status opts
	active string
}

func (t *TUI) newbufPopup(app *tview.Application) {
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

		if _, ok := t.tabs.Get(text); ok {
			t.showError(ErrorExists)
			return
		}

		t.newTab(text, false)
		i := t.tabs.Len() - 1
		t.changeTab(i)
		exit()
	})
}

func (t *TUI) newTab(name string, system bool) {
	num := t.tabs.Len() + 1
	tab := &tab{
		index:    num,
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	// Check for available index
	l := len(t.status.freeIndexes)
	if l > 0 {
		num = t.status.freeIndexes[0]                   // FIFO
		t.status.freeIndexes = t.status.freeIndexes[1:] // Remove
		tab.index = num                                 // Prevents duplication on the slice
	}

	offset := asciiNumbers + num
	if num >= 10 {
		offset = asciiLowercase + (num - 10)
	}

	t.comp.buffers.AddItem(name, "", int32(offset), nil)
	t.tabs.Add(name, tab)
}

func (t *TUI) changeTab(i int) {
	if i < 0 || i >= t.comp.buffers.GetItemCount() {
		return
	}

	t.comp.buffers.SetCurrentItem(i)
	text, _ := t.comp.buffers.GetItemText(i)
	t.ChangeBuffer(text)
}

func (t *TUI) removeTab(name string) {
	b, ok := t.tabs.Get(name)
	if !ok {
		t.showError(ErrorNotFound)
		return
	}

	if b.system {
		t.showError(ErrorSystemBuf)
		return
	}

	if t.active == name {
		// First item (System) will always exist
		t.comp.buffers.SetCurrentItem(0)
		t.ChangeBuffer("System")
	}

	l := t.comp.buffers.FindItems(name, "", true, false)
	for _, v := range l {
		t.comp.buffers.RemoveItem(v)
		t.status.freeIndexes = append(t.status.freeIndexes, b.index) // Available index
	}
	t.tabs.Remove(name)
}
