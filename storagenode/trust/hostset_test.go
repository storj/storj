// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHostSetAdd(t *testing.T) {
	h := NewHostSet()
	require.False(t, h.Add(""))
	require.False(t, h.Add("."))
	require.False(t, h.Add(".."))
	require.True(t, h.Add(".foo."))
	require.True(t, h.Add(".foo"))
	require.True(t, h.Add("foo."))
}

func TestHostSetIncludes(t *testing.T) {
	h := NewHostSet()
	require.True(t, h.Add("foo.test"))
	require.True(t, h.Add("x.bar.test"))
	require.True(t, h.Add(".baz.test"))
	require.True(t, h.Add("satellite"))

	assert.True(t, h.Includes("foo.test"))
	assert.True(t, h.Includes("x.foo.test"))
	assert.True(t, h.Includes(".foo.test"))
	assert.True(t, h.Includes("foo.test."))

	assert.False(t, h.Includes("bar.test"))
	assert.True(t, h.Includes("x.bar.test"))
	assert.True(t, h.Includes("y.x.bar.test"))

	assert.True(t, h.Includes("baz.test"))
	assert.True(t, h.Includes("x.baz.test"))
	assert.True(t, h.Includes(".baz.test"))
	assert.True(t, h.Includes("baz.test."))

	assert.True(t, h.Includes("satellite"))
	assert.False(t, h.Includes("x.satellite"), "satellite is not a domain name so x.satellite should not be included")

	assert.False(t, h.Includes(""))
}
