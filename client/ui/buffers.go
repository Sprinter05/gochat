package ui

import "github.com/Sprinter05/gochat/internal/models"

type tab struct {
	index    int
	messages models.Slice[Message]
	name     string
	system   bool
}

type Buffers struct {
	tabs    models.Table[string, *tab]
	current string
	open    int
	indexes []int
}

func ascii(num int) int32 {
	offset := asciiNumbers + num
	if num >= 10 {
		offset = asciiLowercase + (num - 10)
	}

	return int32(offset)
}

// Returns the currently active tab
func (t *TUI) Buffer() string {
	return t.Active().Buffers().current
}

// Returns the index and asocciated rune unless its hidden
func (b *Buffers) New(name string, system bool) error {
	_, ok := b.tabs.Get(name)
	if ok {
		return ErrorExists
	}

	tab := &tab{
		index:    -1,
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	b.tabs.Add(name, tab)
	return nil
}

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
	l := len(b.indexes)
	if l > 0 {
		t.index = b.indexes[0]    // FIFO
		b.indexes = b.indexes[1:] // Remove
	}

	return b.open - 1, ascii(t.index)
}

func (b *Buffers) Hide(name string) error {
	t, ok := b.tabs.Get(name)
	if !ok {
		return ErrorNotFound
	}

	if t.system {
		return ErrorSystemBuf
	}

	b.open -= 1
	b.indexes = append(b.indexes, t.index)
	t.index = -1
	return nil
}

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
