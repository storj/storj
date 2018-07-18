// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouting(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Routing()

	assert.NotNil(t, result)
}

func TestCache(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Cache()

	assert.NotNil(t, result)
}

func TestMidpoint(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Midpoint()

	assert.Equal(t, "", result)
}

func TestNodes(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Nodes()

	assert.Equal(t, bucket.nodes, result)
}
