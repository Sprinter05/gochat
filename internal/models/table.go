package models

import (
	"sync"
)

/* THREAD SAFE TABLE */

// Table used for storing thread safe maps
type Table[I comparable, T any] struct {
	mut  sync.RWMutex
	data map[I]T
}

/* FUNCTIONS */

// Allocates the data table
func NewTable[I comparable, T any](size int) Table[I, T] {
	return Table[I, T]{
		data: make(map[I]T, size),
	}
}

// Thread safe write
func (t *Table[I, T]) Add(i I, v T) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.data[i] = v
}

// Thread safe write
func (t *Table[I, T]) Remove(i I) {
	t.mut.Lock()
	defer t.mut.Unlock()
	delete(t.data, i)
}

// Thread safe read
func (t *Table[I, T]) Get(i I) (T, bool) {
	t.mut.RLock()
	defer t.mut.RUnlock()
	v, ok := t.data[i]

	if !ok {
		// Empty value of T
		var empty T
		return empty, false
	}

	return v, true
}

// Thread safe read
func (t *Table[I, T]) GetAll() []T {
	l := len(t.data)
	if l == 0 {
		return nil
	}

	// Preallocate
	array := make([]T, 0, l)

	t.mut.RLock()
	defer t.mut.RUnlock()
	for _, v := range t.data {
		array = append(array, v)
	}

	return array
}
