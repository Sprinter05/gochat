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
	last    string
	indexes []int
}

func (b *Buffers) New(name string, system bool) (int, rune, error) {
	_, ok := b.tabs.Get(name)
	if ok {
		return -1, -1, ErrorExists
	}

	num := b.tabs.Len() + 1
	tab := &tab{
		index:    num,
		messages: models.NewSlice[Message](0),
		name:     name,
		system:   system,
	}

	// Check for available index
	l := len(b.indexes)
	if l > 0 {
		num = b.indexes[0]        // FIFO
		b.indexes = b.indexes[1:] // Remove
		tab.index = num           // Prevents duplication on the slice
	}

	offset := asciiNumbers + num
	if num >= 10 {
		offset = asciiLowercase + (num - 10)
	}

	b.tabs.Add(name, tab)
	i := b.tabs.Len() - 1
	return i, int32(offset), nil
}

func (b *Buffers) Remove(name string, indexes []int) error {
	t, ok := b.tabs.Get(name)
	if !ok {
		return ErrorNotFound
	}

	if t.system {
		return ErrorSystemBuf
	}

	b.indexes = append(b.indexes, indexes...)
	b.tabs.Remove(name)
	return nil
}
