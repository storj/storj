// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Package satellitedbhook wraps satellite.DB, so that tests can hook into
// any database method call made with a specific context.
//
// Wrap the database, e.g. with Module in modular tests or via testplanet.Reconfigure.SatelliteDB:
//
//	SatelliteDB: func(_ *zap.Logger, _ int, db satellite.DB) (satellite.DB, error) {
//		return satellitedbhook.Wrap(db), nil
//	},
//
// Then add hooks to the context:
//
//	ctx, done := satellitedbhook.Before[console.Users](ctx, "Get",
//		func(ctx context.Context, id uuid.UUID) error { ... })
//	defer done()
package satellitedbhook

//go:generate go run ./gen

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"

	"storj.io/storj/satellite"
	"storj.io/storj/shared/mud"
)

// Wrap wraps db, so that hooks added with Before and After are called.
func Wrap(db satellite.DB) satellite.DB {
	return wrapSatelliteDB(db)
}

// Module wraps satellite.DB in the ball, so that hooks work in modular tests.
// It must be called after satellite.DB is registered.
func Module(ball *mud.Ball) {
	mud.Decorate[satellite.DB](ball, Wrap)
}

// Before returns a context, which calls fn before every call to method of
// interface T, when made with the returned context.
//
// fn must have the same parameters as the method and return an error,
// e.g. func(ctx context.Context, id uuid.UUID) error for console.Users.Get.
// When fn returns an error, the method is not called and the error is
// returned instead. For methods without an error result, the error causes a panic.
//
// The returned done func disables the hook. It panics with "unused method hook",
// when the hook was never called.
func Before[T any](ctx context.Context, method string, fn any) (_ context.Context, done func()) {
	return add(ctx, reflect.TypeFor[T](), method, fn, false)
}

// After returns a context, which calls fn after every call to method of
// interface T, when made with the returned context.
//
// fn must take the parameters followed by the results of the method and return an error,
// e.g. func(ctx context.Context, id uuid.UUID, user *console.User, err error) error
// for console.Users.Get. The returned error replaces the error result of the method.
// For methods without an error result, a returned error causes a panic.
//
// The returned done func disables the hook. It panics with "unused method hook",
// when the hook was never called.
func After[T any](ctx context.Context, method string, fn any) (_ context.Context, done func()) {
	return add(ctx, reflect.TypeFor[T](), method, fn, true)
}

var (
	contextType = reflect.TypeFor[context.Context]()
	errorType   = reflect.TypeFor[error]()
)

func add(ctx context.Context, iface reflect.Type, method string, fn any, after bool) (_ context.Context, done func()) {
	if iface.Kind() != reflect.Interface {
		panic(fmt.Sprintf("satellitedbhook: %v is not an interface", iface))
	}
	m, ok := iface.MethodByName(method)
	if !ok {
		panic(fmt.Sprintf("satellitedbhook: %v does not have method %q", iface, method))
	}
	if m.Type.NumIn() == 0 || m.Type.In(0) != contextType {
		panic(fmt.Sprintf("satellitedbhook: %v.%s does not take context.Context as first argument", iface, method))
	}

	var in []reflect.Type
	for i := range m.Type.NumIn() {
		in = append(in, m.Type.In(i))
	}
	variadic := m.Type.IsVariadic()
	if after {
		for i := range m.Type.NumOut() {
			in = append(in, m.Type.Out(i))
		}
		variadic = false
	}
	expected := reflect.FuncOf(in, []reflect.Type{errorType}, variadic)
	if reflect.TypeOf(fn) != expected {
		panic(fmt.Sprintf("satellitedbhook: hook for %v.%s must be %v, got %T", iface, method, expected, fn))
	}

	h := &hook{iface: iface, method: method, after: after, fn: reflect.ValueOf(fn)}
	parent, _ := ctx.Value(hooksKey{}).(*hooks)
	ctx = context.WithValue(ctx, hooksKey{}, &hooks{hook: h, next: parent})

	return ctx, func() {
		h.done.Store(true)
		if !h.called.Load() {
			panic(fmt.Sprintf("unused method hook: %v.%s", iface, method))
		}
	}
}

type hooksKey struct{}

// hooks is an immutable list of hooks, so that derived contexts don't affect each other.
type hooks struct {
	hook *hook
	next *hooks
}

type hook struct {
	iface  reflect.Type
	method string
	after  bool
	fn     reflect.Value

	called atomic.Bool
	done   atomic.Bool
}

// matches checks whether h applies to method of any of the ifaces.
// ifaces contains the wrapped interface and the embedded interfaces that declare the method.
func (h *hook) matches(after bool, method string, ifaces []reflect.Type) bool {
	if h.after != after || h.method != method || h.done.Load() {
		return false
	}
	for _, iface := range ifaces {
		if h.iface == iface {
			h.called.Store(true)
			return true
		}
	}
	return false
}

// invoke calls the hook with the values.
func (h *hook) invoke(values []any) error {
	in := make([]reflect.Value, len(values))
	for i, v := range values {
		if v == nil {
			in[i] = reflect.Zero(h.fn.Type().In(i))
		} else {
			in[i] = reflect.ValueOf(v)
		}
	}

	var out []reflect.Value
	if h.fn.Type().IsVariadic() {
		out = h.fn.CallSlice(in)
	} else {
		out = h.fn.Call(in)
	}
	err, _ := out[0].Interface().(error)
	return err
}

// hasHooks is used to skip collecting arguments, when there are no hooks.
func hasHooks(ctx context.Context) bool {
	return ctx.Value(hooksKey{}) != nil
}

// callBefore calls the before hooks in ctx with the method arguments, stopping at the first error.
func callBefore(ctx context.Context, method string, args []any, ifaces ...reflect.Type) error {
	list, _ := ctx.Value(hooksKey{}).(*hooks)
	for ; list != nil; list = list.next {
		if h := list.hook; h.matches(false, method, ifaces) {
			if err := h.invoke(args); err != nil {
				return err
			}
		}
	}
	return nil
}

// callAfter calls the after hooks in ctx with the method arguments and results.
// When the method returns an error, it's the last result and each hook replaces it.
// It returns the final error.
func callAfter(ctx context.Context, method string, args, results []any, returnsErr bool, ifaces ...reflect.Type) error {
	var err error
	if returnsErr {
		err, _ = results[len(results)-1].(error)
	}
	list, _ := ctx.Value(hooksKey{}).(*hooks)
	for ; list != nil; list = list.next {
		h := list.hook
		if !h.matches(true, method, ifaces) {
			continue
		}
		if returnsErr {
			results[len(results)-1] = err
		}
		hookErr := h.invoke(append(append([]any{}, args...), results...))
		if returnsErr {
			err = hookErr
		} else if hookErr != nil {
			panic(hookErr)
		}
	}
	return err
}
