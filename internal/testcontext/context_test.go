// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

func TestBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ctx.Go(func() error {
		time.Sleep(time.Millisecond)
		return nil
	})

	t.Log(ctx.Dir("a", "b", "c"))
	t.Log(ctx.File("a", "w", "c.txt"))
}

func TestMessage(t *testing.T) {
	var subtest test

	ctx := testcontext.NewWithTimeout(&subtest, 50*time.Millisecond)
	ctx.Go(func() error {
		time.Sleep(time.Second)
		return nil
	})
	ctx.Cleanup()

	assert.Contains(t, subtest.errors[0], "Test exceeded timeout")
	assert.Contains(t, subtest.errors[0], "some goroutines are still running")

	assert.Contains(t, subtest.errors[1], "TestMessage")
}

type test struct {
	errors []string
	fatals []string
}

func (t *test) Name() string { return "Example" }
func (t *test) Helper()      {}

func (t *test) Error(args ...interface{}) { t.errors = append(t.errors, fmt.Sprint(args...)) }
func (t *test) Fatal(args ...interface{}) { t.fatals = append(t.fatals, fmt.Sprint(args...)) }
