// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/storj"
)

func TestNewPieceID(t *testing.T) {
	a := storj.NewPieceID()
	assert.NotEmpty(t, a)

	b := storj.NewPieceID()
	assert.NotEqual(t, a, b)
}

func TestPieceID_Encode(t *testing.T) {
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

func TestPieceID_Derive(t *testing.T) {
	a := storj.NewPieceID()
	b := storj.NewPieceID()

	n0 := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion()).ID
	n1 := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion()).ID

	assert.NotEqual(t, a.Derive(n0), a.Derive(n1), "a(n0) != a(n1)")
	assert.NotEqual(t, b.Derive(n0), b.Derive(n1), "b(n0) != b(n1)")
	assert.NotEqual(t, a.Derive(n0), b.Derive(n0), "a(n0) != b(n0)")
	assert.NotEqual(t, a.Derive(n1), b.Derive(n1), "a(n1) != b(n1)")

	// idempotent
	assert.Equal(t, a.Derive(n0), a.Derive(n0), "a(n0)")
	assert.Equal(t, a.Derive(n1), a.Derive(n1), "a(n1)")

	assert.Equal(t, b.Derive(n0), b.Derive(n0), "b(n0)")
	assert.Equal(t, b.Derive(n1), b.Derive(n1), "b(n1)")
}
