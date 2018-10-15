// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"context"
	"testing"

	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/provider"
)

func TestNewPieceID(t *testing.T) {
	t.Run("should return an id string", func(t *testing.T) {
		assert := assert.New(t)
		id := NewPieceID()
		assert.Equal(id.IsValid(), true)
	})

	t.Run("should return a different string on each call", func(t *testing.T) {
		assert := assert.New(t)
		assert.NotEqual(NewPieceID(), NewPieceID())
	})
}

func TestDerivePieceID(t *testing.T) {
	pid := NewPieceID()
	fid, err := newTestIdentity()
	assert.NoError(t, err)
	nid := dht.NodeID(fid.ID)

	did, err := pid.Derive(nid.Bytes())
	assert.NoError(t, err)
	assert.NotEqual(t, pid, did)

	did2, err := pid.Derive(nid.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, did, did2)

	_, err = base58.Decode(did.String())
	assert.NoError(t, err)
}

// helper function to generate new node identities with
// correct difficulty and concurrency
func newTestIdentity() (*provider.FullIdentity, error) {
	ctx := context.Background()
	fid, err := node.NewFullIdentity(ctx, 12, 4)
	return fid, err
}
