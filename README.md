# go-simplepool

Package `simplepool` provides a concurrent, generic, fixed-capacity object pool. It maintains a fixed number of objects throughout the pool's lifetime, reusing the same instances without growing or shrinking based on demand. For a more dynamic variable-capacity alternative, consider https://github.com/michaellenaghan/go-pool.

- The pool maintains `Count` busy and idle objects
- Idle objects are stored in a buffered channel
- Idle objects are reused on a FIFO (first in, first out) basis; in other words, the least recently used object is reused first
- When there are no idle objects, `Get()` calls wait for an object to be returned by `Put()`
- Waiting `Get()` calls are served on a FIFO (first in, first out) basis

Code is available at [github.com/michaellenaghan/go-simplepool](https://github.com/michaellenaghan/go-simplepool).

Documentation is available at [pkg.go.dev/github.com/michaellenaghan/go-simplepool](https://pkg.go.dev/github.com/michaellenaghan/go-simplepool).

## Installation

```bash
go get github.com/michaellenaghan/go-simplepool
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/michaellenaghan/go-simplepool"
)

func main() {
	pool, err := simplepool.New(
		simplepool.Config[int]{
			Count:       10,
			NewFunc:     func() (int, error) { return 0, nil },
			DestroyFunc: func(int) {}, // this is optional, actually
		},
	)
	if err != nil {
		fmt.Printf("Failed to create pool: %v\n", err)
		return
	}
	defer pool.Stop()

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			obj, err := pool.Get(context.Background())
			if err != nil {
				fmt.Printf("Failed to get object: %v\n", err)
				return
			}
			defer pool.Put(obj)

			time.Sleep(10 * time.Millisecond)
		}()
	}
	wg.Wait()
}
```

## License

MIT License