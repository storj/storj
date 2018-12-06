package signal

import (
	"sync"
)

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
func (d *Dispatcher) Dispatch(name string) error {
	d.Lock()
	defer d.Unlock()

	if d.callbacks[name] == nil {
		d.callbacks[name] = make([]Callback, 0)
	}

	for _, cb := range d.callbacks[name] {
		if err := cb(); err != nil {
			return err
		}
	}

	return nil
}

// Register adds a callback on the dispatcher
func (d *Dispatcher) Register(name string, c Callback) {
	d.Lock()
	defer d.Unlock()

	if d.callbacks[name] == nil {
		d.callbacks[name] = make([]Callback, 0)
	}
	d.callbacks[name] = append(d.callbacks[name], c)
}
