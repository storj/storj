// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

var (
	// ErrUtils - console utils error type.
	ErrUtils = errs.Class("console api utils")
)

// ServeJSONError writes a JSON error to the response output stream.
func ServeJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error) {
	serveCustomJSONError(log, w, status, err, err.Error())
}

// serveCustomJSONError writes a JSON error with a custom message to the response output stream.
func serveCustomJSONError(log *zap.Logger, w http.ResponseWriter, status int, err error, msg string) {
	fields := []zap.Field{
		zap.Int("code", status),
		zap.String("message", msg),
		zap.Error(err),
	}
	switch status {
	case http.StatusNoContent:
		return
	case http.StatusInternalServerError:
		log.Error("returning error to client", fields...)
	case http.StatusBadRequest:
		log.Debug("returning error to client", fields...)
	default:
		log.Info("returning error to client", fields...)
	}

	w.WriteHeader(status)

	err = json.NewEncoder(w).Encode(map[string]string{
		"error": msg,
	})
	if err != nil {
		log.Error("failed to write json error response", zap.Error(ErrUtils.Wrap(err)))
	}
}

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
