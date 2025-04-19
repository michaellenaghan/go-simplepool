// Package `simplepool` provides a concurrent, generic, fixed-capacity object
// pool. It maintains a fixed number of objects throughout the pool's lifetime,
// reusing the same instances without growing or shrinking based on demand.
// For a more dynamic variable-capacity alternative, consider
// https://github.com/michaellenaghan/go-pool.
package simplepool

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrNew               = errors.New("failed to make new pool object")
	ErrStoppingOrStopped = errors.New("pool is stopping or has stopped")
)

// Pool is a generic object pool that manages a collection of objects of type T.
type Pool[T any] struct {
	newFunc     func() (T, error) // required
	destroyFunc func(T)           // optional

	idle chan T // cap = count

	stopping chan struct{}
}

// New creates a new object pool.
//
// New checks the provided config by calling config.Check(). If there's an
// error, New returns it.
//
// Otherwise, New immediately creates the pool objects. If there's an error
// creating one of those objects, New destroys the objects it created and
// returns the error.
func New[T any](config Config[T]) (*Pool[T], error) {
	err := config.Check()
	if err != nil {
		return nil, err
	}

	p := &Pool[T]{
		newFunc:     config.NewFunc,
		destroyFunc: config.DestroyFunc,
		idle:        make(chan T, config.Count),
		stopping:    make(chan struct{}),
	}

	for range cap(p.idle) {
		object, err := p.newFunc()
		if err != nil {
			for range len(p.idle) {
				object := <-p.idle
				if p.destroyFunc != nil {
					p.destroyFunc(object)
				}
			}
			return nil, fmt.Errorf("%w: %v", ErrNew, err)
		}
		p.idle <- object
	}

	return p, nil
}

// Stop stops the pool.
//
// If the pool is already stopping, or has already stopped, Stop does nothing.
//
// Otherwise, Stop destroys all idle objects and then waits for all busy
// objects to be destroyed before returning.
func (p *Pool[T]) Stop() {
	select {
	case <-p.stopping:
		return
	default:
		close(p.stopping)
	}

	for range cap(p.idle) {
		object := <-p.idle
		if p.destroyFunc != nil {
			p.destroyFunc(object)
		}
	}
}

// Get returns an object from the pool, and an error.
//
// (If the error is not nil the object will be the zero value of the type T.)
//
// If the pool is stopping or stopped, Get returns an error.
//
// Otherwise, if there are idle objects, Get returns the least recently used
// idle object (FIFO).
//
// Otherwise, Get waits for an object to be returned to the pool by Put.
//
// (Waiting Get calls are served in FIFO order.)
//
// Get stops waiting when the provided context is cancelled or when Stop is
// called.
func (p *Pool[T]) Get(ctx context.Context) (T, error) {
	select {
	case object := <-p.idle:
		return object, nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case <-p.stopping:
		var zero T
		return zero, ErrStoppingOrStopped
	}
}

// Put returns an object to the pool.
func (p *Pool[T]) Put(object T) {
	p.idle <- object
}
