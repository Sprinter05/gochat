package models

import (
	"context"
	"slices"
	"sync"
)

/* CONCURRENTLY SAFE WAITLIST */

// Used to store sorted data and make goroutines
// wait for a specific piece of data under
// certain conditions.
type Waitlist[T any] struct {
	data []T            // actual data
	cond *sync.Cond     // waiting condition
	sort func(T, T) int // sorting for optimisation
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

// Wakes up all waiting threads and cancels the context.
func (w *Waitlist[T]) Cancel(cancel context.CancelFunc) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	cancel()
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
// A context must be passed which will be checked
// whenever the goroutine wakes up and return its
// error if its not nil.
func (w *Waitlist[T]) Get(ctx context.Context, find func(T) bool) (T, error) {
	w.cond.L.Lock()
	defer w.cond.L.Unlock()

	for {
		for i, v := range w.data {
			if find(v) {
				w.data = slices.Delete(w.data, i, i+1)
				return v, nil
			}
		}

		w.cond.Wait()

		if ctx.Err() != nil {
			var empty T
			return empty, ctx.Err()
		}
	}
}
