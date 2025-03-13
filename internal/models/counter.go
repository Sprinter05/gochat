package models

import (
	"errors"
	"sync"
)

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
func NewCounter(max int) Counter {
	return Counter{
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
