// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

func TestCompile(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	exe := ctx.Compile("storj.io/storj/examples/grpc-debug")
	assert.NotEmpty(t, exe)
}
