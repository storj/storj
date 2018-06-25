// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetermineID(t *testing.T) {
	t.Run("should return an id string", func(t *testing.T) {
		assert := assert.New(t)
		id := DetermineID()
		assert.Equal(len(id) >= IDLength, true)
	})

	t.Run("should return a different string on each call", func(t *testing.T) {
		assert := assert.New(t)
		assert.NotEqual(DetermineID(), DetermineID())
	})
}

func TestMain(m *testing.M) {
	m.Run()
}
