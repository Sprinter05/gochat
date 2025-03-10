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

// Allocates the data
func (t *Table[I, T]) Init() {
	t.data = make(map[I]T)
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
type Counter struct {
	mut sync.Mutex
	val int
}

/* FUNCTIONS */

// Returns the value of the counter
func (c *Counter) Get() int {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.val
}

// Increases the value of the counter
func (c *Counter) Inc() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.val++
}

// Decreases the value of the counter
func (c *Counter) Dec() {
	c.mut.Lock()
	defer c.mut.Unlock()
	if c.val > 0 {
		c.val--
	}
}
