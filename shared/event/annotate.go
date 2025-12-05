// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package event

import (
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
)

const nameKey = "name"

type key int

var annotationKey key

// Annotations stores key-value pairs for event tracking with thread-safe access.
type Annotations struct {
	mu sync.RWMutex
	kv map[string]interface{}
}

// Set stores a key-value pair in the annotations.
func (a *Annotations) Set(key string, value interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.kv[key] = value
}

// Get retrieves the value associated with the given key.
func (a *Annotations) Get(key string) any {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.kv[key]
}

// ForEach iterates over all key-value pairs and calls the given function.
func (a *Annotations) ForEach(f func(key string, value interface{})) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for k, v := range a.kv {
		f(k, v)
	}
}

// Copy creates a deep copy of the annotations.
func (a *Annotations) Copy() *Annotations {
	resp := &Annotations{
		kv: make(map[string]interface{}, len(a.kv)),
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for k, v := range a.kv {
		resp.kv[k] = v
	}
	return resp
}

// StartEvent creates a new event context with the given name and returns a context and function to report data at the end.
func StartEvent(ctx context.Context, name string) (context.Context, func()) {
	nctx := context.WithValue(ctx, annotationKey, &Annotations{
		kv: map[string]any{
			nameKey: name,
		},
	})
	span := monkit.SpanFromCtx(nctx)
	if span != nil {
		Annotate(nctx, "trace_id", span.Trace().Id())
	}
	return nctx, func() {
		Report(nctx)
	}
}

// Annotate adds a typed key-value annotation to the event context.
func Annotate[T int64 | string | []byte | int | float64 | bool | time.Time | time.Duration](ctx context.Context, key string, value T) {
	val := ctx.Value(annotationKey)
	if val == nil {
		return
	}
	annotations, ok := val.(*Annotations)
	if !ok {
		return
	}
	annotations.Set(key, value)
}

// ForkEvent creates a new event context by copying annotations from the parent context.
func ForkEvent(ctx context.Context) (context.Context, func()) {
	val := ctx.Value(annotationKey)
	if val == nil {
		return ctx, func() {}
	}
	annotations, ok := val.(*Annotations)
	if !ok {
		return ctx, func() {}
	}
	nctx := context.WithValue(ctx, annotationKey, annotations.Copy())
	return nctx, func() {
		Report(nctx)
	}
}
