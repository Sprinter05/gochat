// This package aims to provide generic data types
// to abstract certain operations that may be
// performed by both server and client.
package models

import (
	"sync"
)

/* CONCURRENTLY SAFE TABLE */

// Table used for storing a map
// that is safe to use concurrently.
type Table[I comparable, T any] struct {
	mut  sync.RWMutex // mutex
	data map[I]T      // actual data
}

/* FUNCTIONS */

// Returns an allocated data table according to provided size.
func NewTable[I comparable, T any](size uint) Table[I, T] {
	return Table[I, T]{
		data: make(map[I]T, size),
	}
}

// Adds an element to the table.
func (t *Table[I, T]) Add(i I, v T) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.data[i] = v
}

// Removes an element from the table, no
// error will be reported if its not found.
func (t *Table[I, T]) Remove(i I) {
	t.mut.Lock()
	defer t.mut.Unlock()
	delete(t.data, i)
}

// Clears all elements from the table.
func (t *Table[I, T]) Clear() {
	t.mut.Lock()
	defer t.mut.Unlock()
	clear(t.data)
}

// Returns an element from the table
// and a boolean specifying if it exists.
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

// Returns the amount of elements present
// in the table
func (t *Table[I, T]) Len() int {
	t.mut.RLock()
	defer t.mut.RUnlock()
	return len(t.data)
}

// Returns all indexes of the table
func (t *Table[I, T]) Indexes() []I {
	l := len(t.data)
	if l == 0 {
		return nil
	}

	array := make([]I, 0, l)

	t.mut.RLock()
	defer t.mut.RUnlock()
	for i := range t.data {
		array = append(array, i)
	}

	return array
}

// Returns all value elements of the
// table in an array. It is important
// to note that order can change.
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
