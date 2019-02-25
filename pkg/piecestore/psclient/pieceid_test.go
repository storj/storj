// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"testing"

	"github.com/mr-tron/base58/base58"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
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
	ctx := testcontext.New(t)
	pid := NewPieceID()
	fid, err := testidentity.NewTestIdentity(ctx)
	assert.NoError(t, err)
	did, err := pid.Derive(fid.ID.Bytes())
	assert.NoError(t, err)
	assert.NotEqual(t, pid, did)

	did2, err := pid.Derive(fid.ID.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, did, did2)

	_, err = base58.Decode(did.String())
	assert.NoError(t, err)
}
