// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pgutil_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/dbutil/pgutil"
)

// errorReader returns error for the specified number of Read operations.
type errorReader struct {
	numberOfRetries int
	succeedOnRetry  int
}

// Read generates a fake error for testing error conditions.
func (r errorReader) Read(p []byte) (n int, err error) {
	r.numberOfRetries++
	if r.succeedOnRetry > 0 && r.numberOfRetries >= r.succeedOnRetry {
		return 10, nil
	}
	return 0, fmt.Errorf("Fake rand() error")
}

// TestRandomString tests to verify that by default we get a proper
// randomized string for the specified length we expect.
func TestRandomString(t *testing.T) {
	s := pgutil.RandomString(10)
	assert.NotEmpty(t, s)
}

// TestRandomStringError verifies that given an error reading from crypto/rand
// that we correctly get the panic we expect.
func TestRandomStringError(t *testing.T) {
	r := errorReader{}
	assert.Panics(t, func() { pgutil.RandomStringFromReader(10, r) })
}

// TestRandomStringSucceedsAfterOneRetry verifies that given an error reading
// from crypto/rand that we correctly get the success we expect after 1 retry.
func TestRandomStringSucceedsAfterOneRetry(t *testing.T) {
	r := errorReader{succeedOnRetry: 1}
	assert.NotPanics(t, func() { pgutil.RandomStringFromReader(10, r) })
}
