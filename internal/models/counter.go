package models

import (
	"errors"
	"sync"
)

/* CONCURRENTLY SAFE COUNTER */

// Counter that can increase and decrease
// up to a certain maximum value. It is implemented
// so that it is safe to use concurrently.
type Counter struct {
	cond *sync.Cond // waiting condition
	val  int        // current value of counter
	max  int        // max allowed value of countter
	last int        // amount of waiting goroutines
}

/* FUNCTIONS */

// Creates a new counter with the maximum value it can have.
func NewCounter(max int) Counter {
	return Counter{
		max:  max,
		cond: sync.NewCond(new(sync.Mutex)),
	}
}

// Returns the current value of the counter.
func (c *Counter) Get() int {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	return c.val
}

// Increases the value of the counter
// Will block the goroutine when the value
// is max until it can be increased again.
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
			// We decrease on all waiting goroutines
			// That way it works like a priority queue
			pos--
		}
	}

	c.val++
}

// Tries to increase the value, if the value is max
// an error will be returned.
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
// Notifies anyone waiting to increase
// the counter again.
func (c *Counter) Dec() {
	c.cond.L.Lock()
	defer c.cond.L.Unlock()
	if c.val > 0 {
		c.val--
	}

	// Notify all waiting goroutines
	c.cond.Broadcast()
}
