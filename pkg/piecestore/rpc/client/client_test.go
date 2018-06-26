// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestMain(m *testing.M) {
	m.Run()
}
