package models

import (
	"slices"
	"sync"
)

/* CONCURRENTLY SAFE WAITLIST */

// Used to store sorted data and make goroutines
// wait for a specific piece of data under
// certain conditions.
type Waitlist[T any] struct {
	data []T
	cond *sync.Cond
	sort func(T, T) int
}

// Returns a preallocated slice with 0 elements according to the given
// capacity and also sets the function that will sort elements
// according to [slices.SortFunc].
func NewWaitlist[T any](cap uint, sort func(T, T) int) Waitlist[T] {
	return Waitlist[T]{
		data: make([]T, 0, cap),
		cond: sync.NewCond(new(sync.Mutex)),
		sort: sort,
	}
}

// Inserts an element into the waitlist, sorts
// the list and notifies all waiting goroutines.
func (w *Waitlist[T]) Insert(v T) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	w.data = append(w.data, v)
	slices.SortFunc(w.data, w.sort)

	w.cond.Broadcast()
}

// Clears all elements from the waitlist.
func (w *Waitlist[T]) Clear() {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()
	clear(w.data)
}

// Tries to retrieve an element from the waitlist that
// fulfills the given function and returns true/false
// depending on if the element was or not found.
func (w *Waitlist[T]) TryGet(find func(T) bool) (T, bool) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	for i, v := range w.data {
		if find(v) {
			w.data = slices.Delete(w.data, i, i+1)
			return v, true
		}
	}

	var empty T
	return empty, false
}

// Tries to retrieve an element that fulfills
// the given function. If the element is not found
// the caller goroutine will sleep and wake up
// when a new element is inserted, repeating
// this process forever until the element is found.
func (w *Waitlist[T]) Get(find func(T) bool) T {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	for {
		for i, v := range w.data {
			if find(v) {
				w.data = slices.Delete(w.data, i, i+1)
				return v
			}
		}

		w.cond.Wait()
	}
}
