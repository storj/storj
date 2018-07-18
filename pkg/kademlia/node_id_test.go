// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestString(t *testing.T) {
	expected := "test node"
	node := NodeID(expected)

	result := node.String()

	assert.Equal(t, expected, result)
}

func TestStringToNodeID(t *testing.T) {
	str := "test node"
	node := NodeID(str)
	expected := StringToNodeID(str)

	assert.Equal(t, expected.String(), node.String())
}

func TestNewID(t *testing.T) {
	_, err := NewID()
	assert.NoError(t, err)
}
