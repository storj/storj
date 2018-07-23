// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TODO: tests and corresponding methods are incomplete. Make sure to update. 7/20/18
func TestRouting(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Routing()

	assert.NotNil(t, result)
}

//TODO: tests and corresponding methods are incomplete. Make sure to update. 7/20/18
func TestCache(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Cache()

	assert.NotNil(t, result)
}

//TODO: tests and corresponding methods are incomplete. Make sure to update. 7/20/18
func TestMidpoint(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Midpoint()

	assert.Equal(t, "", result)
}

//TODO: tests and corresponding methods are incomplete. Make sure to update. 7/20/18
func TestNodes(t *testing.T) {
	bucket := &KBucket{}
	result := bucket.Nodes()

	assert.Equal(t, bucket.nodes, result)
}
