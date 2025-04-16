package simplepool

import "errors"

// Config configures a new pool.
type Config[T any] struct {
	// Count is the number of objects in the pool.
	// Must be >= 0.
	Count int
	// NewFunc is a function that creates a new object.
	// This function is required.
	NewFunc func() (T, error)
	// DestroyFunc is a function that destroys an object when it's no longer
	// needed.
	// This function is optional.
	DestroyFunc func(T)
}

// Check checks the configuration.
//
// If the configuration is invalid, Check returns an error.
func (c *Config[T]) Check() error {
	if c.Count < 0 {
		return errors.New("count must be greater than or equal to zero")
	}
	if c.NewFunc == nil {
		return errors.New("newFunc is required")
	}
	return nil
}
