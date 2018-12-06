// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package signal

import (
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

// ErrNilCallback throws if there is no callback function provided
var ErrNilCallback = errs.Class("must provide a callback function to register")

// Dispatcher is what's loaded onto each service
type Dispatcher struct {
	sync.RWMutex
	source    string
	callbacks map[string][]Callback
}

// Callback is the general type for functions in this package
type Callback func() error

// NewDispatcher is what each service calls to create a dispatch service
func NewDispatcher(source string) *Dispatcher {
	return &Dispatcher{
		source:    source,
		callbacks: make(map[string][]Callback),
	}
}

// Dispatch broadcasts an event across all callbacks
// If any of them error, it will return that error,
func (d *Dispatcher) Dispatch(name string) error {
	d.Lock()
	defer d.Unlock()

	errors := []error{}

	if d.callbacks[name] == nil {
		d.callbacks[name] = make([]Callback, 0)
	}

	for _, cb := range d.callbacks[name] {
		if err := cb(); err != nil {
			errors = append(errors, err)
		}
	}

	return utils.CombineErrors(errors...)
}

// Register adds a callback on the dispatcher
func (d *Dispatcher) Register(name string, c Callback) error {
	d.Lock()
	defer d.Unlock()

	if c == nil {
		return ErrNilCallback.New("")
	}

	if d.callbacks[name] == nil {
		d.callbacks[name] = make([]Callback, 0)
	}
	d.callbacks[name] = append(d.callbacks[name], c)
	return nil
}
