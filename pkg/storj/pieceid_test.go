// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
)

func TestNewPieceID(t *testing.T) {
	a := storj.NewPieceID()
	assert.NotEmpty(t, a)

	b := storj.NewPieceID()
	assert.NotEqual(t, a, b)
}

func TestEncode(t *testing.T) {
	_, err := storj.PieceIDFromString("likn43kilfzd")
	assert.Error(t, err)

	_, err = storj.PieceIDFromBytes([]byte{1, 2, 3, 4, 5})
	assert.Error(t, err)

	for i := 0; i < 10; i++ {
		pieceid := storj.NewPieceID()

		fromString, err := storj.PieceIDFromString(pieceid.String())
		assert.NoError(t, err)
		fromBytes, err := storj.PieceIDFromBytes(pieceid.Bytes())
		assert.NoError(t, err)

		assert.Equal(t, pieceid, fromString)
		assert.Equal(t, pieceid, fromBytes)
	}
}

func TestDerivePieceID(t *testing.T) {
	pieceid := storj.NewPieceID()
	a := testplanet.MustPregeneratedIdentity(0).ID
	b := testplanet.MustPregeneratedIdentity(1).ID

	apieceid := pieceid.Derive(a)
	bpieceid := pieceid.Derive(b)

	assert.NotEqual(t, apieceid, bpieceid)

	assert.Equal(t, apieceid, pieceid.Derive(a))
	assert.Equal(t, bpieceid, pieceid.Derive(b))
}
