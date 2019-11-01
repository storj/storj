// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package groupcancel

import (
	"context"
	"testing"
)

func TestContext_SuccessThreshold(t *testing.T) {
	ctx, cancel := NewContext(context.Background(), 10, .5, 0)
	defer cancel()

	for i := 0; i < 4; i++ {
		ctx.Success()
		select {
		case <-ctx.Done():
			t.FailNow()
		default:
		}
	}

	ctx.Success()
	<-ctx.Done()
}

func TestContext_FailThreshold(t *testing.T) {
	ctx, cancel := NewContext(context.Background(), 10, .5, 0)
	defer cancel()

	for i := 0; i < 4; i++ {
		ctx.Success()
		select {
		case <-ctx.Done():
			t.FailNow()
		default:
		}
	}

	ctx.Failure()
	ctx.Failure()
	<-ctx.Done()
}

func TestContext_AllFailures(t *testing.T) {
	ctx, cancel := NewContext(context.Background(), 10, .5, 0)
	defer cancel()

	for i := 0; i < 9; i++ {
		ctx.Failure()
		select {
		case <-ctx.Done():
			t.FailNow()
		default:
		}
	}

	ctx.Failure()
	<-ctx.Done()
}

func TestContext_UseAfterDone(t *testing.T) {
	ctx, cancel := NewContext(context.Background(), 10, .5, 0)
	defer cancel()

	for i := 0; i < 20; i++ {
		ctx.Success()
		ctx.Failure()
	}

	<-ctx.Done()
}
