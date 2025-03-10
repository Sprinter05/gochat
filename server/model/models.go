package model

import (
	"errors"
	"sync"
	"time"
)

/* GLOBAL DATA TYPES */

// Specifies a permission
type Permission int8

const (
	USER Permission = iota
	ADMIN
	OWNER
)

// Has to conform to UsernameSize on the specification
type Username string

// Specifies a message to be received
type Message struct {
	Sender  Username
	Content []byte
	Stamp   time.Time
}

/* INTERNAL ERRORS */

var (
	ErrorDeregistered        error = errors.New("user has been deregistered")
	ErrorDoesNotExist        error = errors.New("data does not exist")
	ErrorSessionExists       error = errors.New("user is already logged in")
	ErrorDuplicatedSession   error = errors.New("user is logged in from another endpoint")
	ErrorProhibitedOperation error = errors.New("operation trying to be performed is invalid")
	ErrorNoAccount           error = errors.New("user tried performing an operation with no account")
	ErrorDBConstraint        error = errors.New("database returned constraint on operation")
	ErrorNoMessages          error = errors.New("user has no messages to receive")
	ErrorInvalidValue        error = errors.New("data provided is invalid")
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

	array := make([]T, l)
	i := 0

	t.mut.RLock()
	defer t.mut.RUnlock()
	for _, v := range t.data {
		array[i] = v
		i++
	}

	return array
}

/* THREAD SAFE COUNTER */

// Global counter for the amount of clients
// Includes a priority system for waiting
type Counter struct {
	cond *sync.Cond
	val  int
	last int
	max  int
}

/* FUNCTIONS */

// Creates a new counter with the max value it can have
func NewCounter(max int) *Counter {
	return &Counter{
		max:  max,
		cond: sync.NewCond(new(sync.Mutex)),
	}
}

// Returns the value of the counter
func (c *Counter) Get() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return c.val
}

// Increases the value of the counter
// Will block when the value is max
func (c *Counter) Inc() {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	if c.val == c.max {
		// Identifies the priority in the queue
		// last + 1 ensures we wait at least once
		c.last++
		pos := c.last

		// Wait until its our turn
		for pos > 0 {
			c.cond.Wait()
			// Waking up means someone broadcasted
			pos--
		}
	}

	c.val++
}

// Tries to increase the value unless it is max
func (c *Counter) TryInc() error {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	if c.val == c.max {
		return errors.New("counter is at max value")
	}

	// Otherwise we increase the value
	c.val++
	return nil
}

// Decreases the value of the counter
// Notifies all waiting goroutines
func (c *Counter) Dec() {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	if c.val > 0 {
		c.val--
	}

	// Notify all waiting goroutines
	c.cond.Broadcast()
}
