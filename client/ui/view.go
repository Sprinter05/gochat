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
	showUsers   bool
	showBufs    bool
	creatingBuf bool
	freeIndex   []int
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
		SetLabel("Enter buffer name: ").
		SetFieldBackgroundColor(tcell.ColorDefault)
	input.SetBorder(false).
		SetBackgroundColor(tcell.ColorDefault)

	t.area.chat.ResizeItem(t.comp.input, 0, 0)
	t.area.chat.AddItem(input, 1, 0, true)
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
		t.comp.buffers.SetCurrentItem(t.tabs.Len() - 1)
		t.ChangeBuffer(text)
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
	l := len(t.status.freeIndex)
	if l > 0 {
		num = t.status.freeIndex[0]                 // FIFO
		t.status.freeIndex = t.status.freeIndex[1:] // Remove
		tab.index = num                             // Prevents duplication on the slice
	}

	offset := asciiNumbers + num
	if num >= 10 {
		offset = asciiLowercase + (num - 10)
	}

	t.comp.buffers.AddItem(name, "", int32(offset), nil)
	t.tabs.Add(name, tab)
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
		t.status.freeIndex = append(t.status.freeIndex, b.index) // Available index
	}
	t.tabs.Remove(name)
}
