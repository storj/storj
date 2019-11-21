package main

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitmask(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				mask        bitmask
			)

			err := mask.Set(expectedIdx)
			require.NoError(t, err)
		})

		t.Run("error: negative index", func(t *testing.T) {
			var (
				invalidIdx = -(rand.Intn(math.MaxInt32-1) + 1)
				mask       bitmask
			)

			err := mask.Set(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitmaskInvalidIdx.Has(err), "errorBitmaskInvalidIdx class")
		})

		t.Run("error: index > 63", func(t *testing.T) {
			var (
				invalidIdx = rand.Intn(math.MaxInt16) + 64
				mask       bitmask
			)

			err := mask.Set(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitmaskInvalidIdx.Has(err), "errorBitmaskInvalidIdx class")
		})
	})

	t.Run("Has", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				mask        bitmask
			)

			has, err := mask.Has(expectedIdx)
			require.NoError(t, err)
			assert.False(t, has)
		})

		t.Run("error: negative index", func(t *testing.T) {
			var (
				invalidIdx = -(rand.Intn(math.MaxInt32-1) + 1)
				mask       bitmask
			)

			_, err := mask.Has(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitmaskInvalidIdx.Has(err), "errorBitmaskInvalidIdx class")
		})

		t.Run("error: index > 63", func(t *testing.T) {
			var (
				invalidIdx = rand.Intn(math.MaxInt16) + 64
				mask       bitmask
			)

			_, err := mask.Has(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitmaskInvalidIdx.Has(err), "errorBitmaskInvalidIdx class")
		})
	})

	t.Run("Set and Has", func(t *testing.T) {
		var (
			expectedIdx = rand.Intn(64)
			mask        bitmask
		)

		has, err := mask.Has(expectedIdx)
		require.NoError(t, err, "Has")
		assert.False(t, has, "expected tracked index")

		err = mask.Set(expectedIdx)
		require.NoError(t, err, "Set")

		has, err = mask.Has(expectedIdx)
		require.NoError(t, err, "Has")
		assert.True(t, has, "expected tracked index")
	})
}
