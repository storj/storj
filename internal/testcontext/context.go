// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/sync/errgroup"
)

// Context is a
type Context struct {
	context.Context
	group *errgroup.Group
	test  testing.TB

	once      sync.Once
	directory string
}

// New creates a new test context
func New(test testing.TB) *Context {
	group, ctx := errgroup.WithContext(context.Background())
	return &Context{
		Context: ctx,
		group:   group,
		test:    test,
	}
}

// Go runs fn in a goroutine.
// Call Wait to check the result
func (ctx *Context) Go(fn func() error) {
	ctx.test.Helper()
	ctx.group.Go(fn)
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
	_ = os.MkdirAll(dir, 0644)
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
	err := ctx.group.Wait()
	if err != nil {
		ctx.test.Fatal(err)
	}
}

// deleteTemporary tries to delete temporary directory
func (ctx *Context) deleteTemporary() {
	err := os.RemoveAll(ctx.directory)
	if err != nil {
		ctx.test.Fatal(err)
	}
}
