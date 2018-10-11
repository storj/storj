// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	expected := "test node"
	node := ID(expected)

	result := node.String()

	assert.Equal(t, expected, result)
}

func TestIDFromString(t *testing.T) {
	str := "test node"
	node := ID(str)
	expected := IDFromString(str)

	assert.Equal(t, expected.String(), node.String())
}

func TestNewFullIdentity(t *testing.T) {
	_, err := NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)
}
