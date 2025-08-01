package models

import (
	"slices"
	"sync"
)

/* CONCURRENTLY SAFE SLICE */

// Used to store a slice that is safe to
// be accessed concurrently. Stored values
// must be able to be compared together.
type Slice[T comparable] struct {
	mut  sync.RWMutex // mutex
	data []T          // actual data
}

/* FUNCTIONS */

// Returns a preallocated slice with 0 elements
// according to the given capacity.
func NewSlice[T comparable](cap uint) Slice[T] {
	return Slice[T]{
		data: make([]T, 0, cap),
	}
}

// Returns the amount of elements present
// in the slice.
func (s *Slice[T]) Len() int {
	s.mut.Lock()
	defer s.mut.Unlock()
	return len(s.data)
}

// Appends a new element to the slice,
// reallocating it if necessary.
func (s *Slice[T]) Add(v T) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.data = append(s.data, v)
}

// Removes an element from the slice by
// reallocating the indexes, which means
// that, depending on the size of the slice,
// it might be a costly operation.
func (s *Slice[T]) Remove(val T) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.data = slices.DeleteFunc(s.data, func(v T) bool {
		return v == val
	})
}

// Returns the element located at a certain index and
// a boolean indicating if it exists
// (array indexing starts at 0).
func (s *Slice[T]) Get(index uint) (T, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()

	// Out of bounds
	if index >= uint(len(s.data)) {
		var empty T
		return empty, false
	}

	return s.data[index], true
}

// Returns the element found that fulfills the given
// function and a boolean indicating whether it was or
// not found.
func (s *Slice[T]) Find(find func(T) bool) (T, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()

	i := slices.IndexFunc(s.data, find)
	if i == -1 {
		var empty T
		return empty, false
	}

	return s.data[i], true
}

// Clears all elements from the slice.
func (s *Slice[T]) Clear() {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.data = make([]T, 0)
}

// Returns true if the given element exists
// in the slice, returns false otherwise.
func (s *Slice[T]) Has(v T) bool {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return slices.Contains(s.data, v)
}

// Returns a copy of the actual slice data so that it
// can be safely traversed by a single goroutine. An
// optional argument of how many elements to retrieve
// can be provided. To return all elements, "n" must
// be 0. If the slice is empty or "n" goes out of
// bounds, nil will be returned.
func (s *Slice[T]) Copy(n uint) []T {
	s.mut.RLock()
	defer s.mut.RUnlock()
	len := len(s.data)
	if len == 0 {
		return nil
	}

	// Return all elements
	if n == 0 {
		dest := make([]T, 0, len)
		dest = append(dest, s.data...)
		return dest
	}

	// Out of bounds
	if int(n) > len {
		return nil
	}

	// Returns "n" elements
	dest := make([]T, 0, n)
	dest = append(dest, s.data[:n]...)
	return dest
}
