// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
)

const defaultTimeout = 3 * time.Minute

// Context is a context that has utility methods for testing and waiting for asynchronous errors.
type Context struct {
	context.Context

	timedctx context.Context
	cancel   context.CancelFunc

	group *errgroup.Group
	test  TB

	once      sync.Once
	directory string

	mu      sync.Mutex
	running []caller
}

type caller struct {
	pc   uintptr
	file string
	line int
	ok   bool
	done bool
}

// TB is a subset of testing.TB methods
type TB interface {
	Name() string
	Helper()
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// New creates a new test context
func New(test TB) *Context {
	return NewWithTimeout(test, defaultTimeout)
}

// NewWithTimeout creates a new test context with a given timeout
func NewWithTimeout(test TB, timeout time.Duration) *Context {
	timedctx, cancel := context.WithTimeout(context.Background(), timeout)
	group, errctx := errgroup.WithContext(timedctx)

	ctx := &Context{
		Context:  errctx,
		timedctx: timedctx,
		cancel:   cancel,
		group:    group,
		test:     test,
	}

	return ctx
}

// Go runs fn in a goroutine.
// Call Wait to check the result
func (ctx *Context) Go(fn func() error) {
	ctx.test.Helper()

	pc, file, line, ok := runtime.Caller(1)
	ctx.mu.Lock()
	index := len(ctx.running)
	ctx.running = append(ctx.running, caller{pc, file, line, ok, false})
	ctx.mu.Unlock()

	ctx.group.Go(func() error {
		defer func() {
			ctx.mu.Lock()
			ctx.running[index].done = true
			ctx.mu.Unlock()
		}()
		return fn()
	})
}

// Check calls fn and checks result
func (ctx *Context) Check(fn func() error) {
	ctx.test.Helper()
	err := fn()
	if err != nil {
		ctx.test.Fatal(err)
	}
}

// Dir returns a directory path inside temp
func (ctx *Context) Dir(subs ...string) string {
	ctx.test.Helper()

	ctx.once.Do(func() {
		var err error
		ctx.directory, err = ioutil.TempDir("", ctx.test.Name())
		if err != nil {
			ctx.test.Fatal(err)
		}
	})

	dir := filepath.Join(append([]string{ctx.directory}, subs...)...)
	_ = os.MkdirAll(dir, 0744)
	return dir
}

// File returns a filepath inside temp
func (ctx *Context) File(subs ...string) string {
	ctx.test.Helper()

	if len(subs) == 0 {
		ctx.test.Fatal("expected more than one argument")
	}

	dir := ctx.Dir(subs[:len(subs)-1]...)
	return filepath.Join(dir, subs[len(subs)-1])
}

// Cleanup waits everything to be completed,
// checks errors and tries to cleanup directories
func (ctx *Context) Cleanup() {
	ctx.test.Helper()

	defer ctx.deleteTemporary()
	defer ctx.cancel()

	alldone := make(chan error, 1)
	go func() {
		alldone <- ctx.group.Wait()
		defer close(alldone)
	}()

	select {
	case <-ctx.timedctx.Done():
		ctx.reportRunning()
	case err := <-alldone:
		if err != nil {
			ctx.test.Fatal(err)
		}
	}
}

func (ctx *Context) reportRunning() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	var problematic []caller
	for _, caller := range ctx.running {
		if !caller.done {
			problematic = append(problematic, caller)
		}
	}

	var message strings.Builder
	message.WriteString("Test exceeded timeout")
	if len(problematic) > 0 {
		message.WriteString("\nsome goroutines are still running, did you forget to shut them down?")
		for _, caller := range problematic {
			fnname := ""
			if fn := runtime.FuncForPC(caller.pc); fn != nil {
				fnname = fn.Name()
			}
			fmt.Fprintf(&message, "\n%s:%d: %s", caller.file, caller.line, fnname)
		}
	}

	ctx.test.Error(message.String())

	stack := make([]byte, 1*memory.MB.Int())
	n := runtime.Stack(stack, true)
	stack = stack[:n]
	ctx.test.Error("Full Stack Trace:\n", string(stack))
}

// deleteTemporary tries to delete temporary directory
func (ctx *Context) deleteTemporary() {
	if ctx.directory == "" {
		return
	}
	err := os.RemoveAll(ctx.directory)
	if err != nil {
		ctx.test.Fatal(err)
	}
	ctx.directory = ""
}
