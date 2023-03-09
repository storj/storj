// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
)

var (
	// ErrUtils - console utils error type.
	ErrUtils = errs.Class("console api utils")
)

// ContextChannel is a generic, context-aware channel.
type ContextChannel struct {
	mu          sync.Mutex
	channel     chan interface{}
	initialized bool
}

// Get waits until a value is sent and returns it, or returns an error if the context has closed.
func (c *ContextChannel) Get(ctx context.Context) (interface{}, error) {
	c.initialize()
	select {
	case val := <-c.channel:
		return val, nil
	default:
		select {
		case <-ctx.Done():
			return nil, ErrUtils.New("context closed")
		case val := <-c.channel:
			return val, nil
		}
	}
}

// Send waits until a value can be sent and sends it, or returns an error if the context has closed.
func (c *ContextChannel) Send(ctx context.Context, val interface{}) error {
	c.initialize()
	select {
	case c.channel <- val:
		return nil
	default:
		select {
		case <-ctx.Done():
			return ErrUtils.New("context closed")
		case c.channel <- val:
			return nil
		}
	}
}

func (c *ContextChannel) initialize() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.initialized {
		return
	}
	c.channel = make(chan interface{})
	c.initialized = true
}
