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

// DefaultTimeout is the default timeout used by new context
const DefaultTimeout = 3 * time.Minute

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

	Log(args ...interface{})
	Error(args ...interface{})
	Fatal(args ...interface{})
}

// New creates a new test context with default timeout
func New(test TB) *Context {
	return NewWithTimeout(test, DefaultTimeout)
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

// Wait blocks until all of the goroutines launched with Go are done and
// fails the test if any of them returned an error.
func (ctx *Context) Wait() {
	ctx.test.Helper()
	err := ctx.group.Wait()
	if err != nil {
		ctx.test.Fatal(err)
	}
}

// Check calls fn and checks result
func (ctx *Context) Check(fn func() error) {
	ctx.test.Helper()
	err := fn()
	if err != nil {
		ctx.test.Fatal(err)
	}
}

// Dir creates a subdirectory inside temp joining any number of path elements
// into a single path and return its absolute path.
func (ctx *Context) Dir(elem ...string) string {
	ctx.test.Helper()

	ctx.once.Do(func() {
		sanitized := strings.Map(func(r rune) rune {
			if ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') || r == '-' {
				return r
			}
			return '_'
		}, ctx.test.Name())

		var err error
		ctx.directory, err = ioutil.TempDir("", sanitized)
		if err != nil {
			ctx.test.Fatal(err)
		}
	})

	dir := filepath.Join(append([]string{ctx.directory}, elem...)...)
	err := os.MkdirAll(dir, 0744)
	if err != nil {
		ctx.test.Fatal(err)
	}
	return dir
}

// File returns a filepath inside a temp directory joining any number of path
// elements into a single path and returns its absolute path.
func (ctx *Context) File(elem ...string) string {
	ctx.test.Helper()

	if len(elem) == 0 {
		ctx.test.Fatal("expected more than one argument")
	}

	dir := ctx.Dir(elem[:len(elem)-1]...)
	return filepath.Join(dir, elem[len(elem)-1])
}

// Cleanup waits everything to be completed,
// checks errors and goroutines which haven't ended and tries to cleanup
// directories
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

	stack := make([]byte, 1*memory.MiB)
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
