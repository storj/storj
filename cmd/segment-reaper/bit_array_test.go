// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBitArray(t *testing.T) {
	t.Run("Set", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				bits        bitArray
			)

			err := bits.Set(expectedIdx)
			require.NoError(t, err)
		})

		t.Run("error: negative index", func(t *testing.T) {
			var (
				invalidIdx = -(rand.Intn(math.MaxInt32-1) + 1)
				bits       bitArray
			)

			err := bits.Set(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitArrayInvalidIdx.Has(err), "errorBitArrayInvalidIdx class")
		})

		t.Run("error: index > 63", func(t *testing.T) {
			var (
				invalidIdx = rand.Intn(math.MaxInt16) + 64
				bits       bitArray
			)

			err := bits.Set(invalidIdx)
			assert.NoError(t, err)
			assert.False(t, errorBitArrayInvalidIdx.Has(err), "errorBitArrayInvalidIdx class")
		})
	})

	t.Run("Has", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				bits        bitArray
			)

			has, err := bits.Has(expectedIdx)
			require.NoError(t, err)
			assert.False(t, has)
		})

		t.Run("error: negative index", func(t *testing.T) {
			var (
				invalidIdx = -(rand.Intn(math.MaxInt32-1) + 1)
				bits       bitArray
			)

			_, err := bits.Has(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitArrayInvalidIdx.Has(err), "errorBitArrayInvalidIdx class")
		})
	})

	t.Run("Set and Has", func(t *testing.T) {
		t.Run("index not set", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				bits        bitArray
			)

			has, err := bits.Has(expectedIdx)
			require.NoError(t, err, "Has")
			assert.False(t, has, "expected tracked index")
		})

		t.Run("index is set", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(64)
				bits        bitArray
			)

			err := bits.Set(expectedIdx)
			require.NoError(t, err, "Set")

			has, err := bits.Has(expectedIdx)
			require.NoError(t, err, "Has")
			assert.True(t, has, "expected tracked index")
		})

		t.Run("same index is set more than once", func(t *testing.T) {
			var (
				expectedIdx = rand.Intn(63)
				times       = rand.Intn(10) + 2
				bits        bitArray
			)

			for i := 0; i < times; i++ {
				err := bits.Set(expectedIdx)
				require.NoError(t, err, "Set")
			}

			has, err := bits.Has(expectedIdx)
			require.NoError(t, err, "Has")
			assert.True(t, has, "expected tracked index")

			// Another index isn't set
			has, err = bits.Has(expectedIdx + 1)
			require.NoError(t, err, "Has")
			assert.False(t, has, "not expected tracked index")
		})

		t.Run("several indexes are set", func(t *testing.T) {
			var (
				numIndexes = rand.Intn(61) + 2
				indexes    = make([]int, numIndexes)
				bits       bitArray
			)

			for i := 0; i < numIndexes; i++ {
				idx := rand.Intn(63)
				indexes[i] = idx

				err := bits.Set(idx)
				require.NoError(t, err, "Set")
			}

			for _, idx := range indexes {
				has, err := bits.Has(idx)
				require.NoError(t, err, "Has")
				assert.True(t, has, "expected tracked index")
			}
		})
	})

	t.Run("Count", func(t *testing.T) {
		t.Run("when initialized", func(t *testing.T) {
			var bits bitArray

			numIndexes := bits.Count()
			assert.Zero(t, numIndexes)
		})

		t.Run("when several indexes set", func(t *testing.T) {
			var (
				numSetCalls        = rand.Intn(61) + 2
				expectedNumIndexes = numSetCalls
				bits               bitArray
			)

			for i := 0; i < numSetCalls; i++ {
				idx := rand.Intn(63)

				ok, err := bits.Has(idx)
				require.NoError(t, err, "Has")
				if ok {
					// idx was already set in previous iteration
					expectedNumIndexes--
					continue
				}

				err = bits.Set(idx)
				require.NoError(t, err, "Set")
			}

			numIndexes := bits.Count()
			assert.Equal(t, expectedNumIndexes, numIndexes)
		})
	})

	t.Run("IsSequence", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			var bits bitArray

			ok := bits.IsSequence()
			assert.True(t, ok)
		})

		t.Run("no sequence", func(t *testing.T) {
			var bits bitArray

			for { // loop until getting a list of non-sequenced indexes
				var (
					numIndexes = rand.Intn(60) + 2
					indexes    = make([]int, numIndexes)
				)

				for i := 0; i < numIndexes; i++ {
					idx := rand.Intn(63)
					indexes[i] = idx
				}

				sort.Ints(indexes)

				areSequenced := true
				for i, idx := range indexes {
					if i > 0 && (indexes[i-1]-1) < idx {
						areSequenced = false
					}
					err := bits.Set(idx)
					require.NoError(t, err, "Set")
				}

				if !areSequenced {
					break
				}
			}

			ok := bits.IsSequence()
			assert.False(t, ok)
		})

		testCases := []struct {
			name       string
			startIndex int
			numIndexes int
			isSequence bool
		}{
			{
				name:       "sequence starts at index 0",
				startIndex: 0,
				numIndexes: rand.Intn(5000) + 1,
				isSequence: true,
			},
			{
				name:       "sequence starts at index 8 until index 15",
				startIndex: 8,
				numIndexes: 15,
				isSequence: false,
			},
			{
				name:       "sequence starts at index 8 until index 16",
				startIndex: 8,
				numIndexes: 16,
				isSequence: false,
			},
			{
				name:       "sequence starts at index 8 until index 17",
				startIndex: 8,
				numIndexes: 17,
				isSequence: false,
			},
			{
				name:       "sequence starts at index 8 until index 23",
				startIndex: 8,
				numIndexes: 23,
				isSequence: false,
			},
			{
				name:       "sequence starts at other index than 0",
				startIndex: rand.Intn(1000) + 1,
				numIndexes: rand.Intn(5000) + 1002,
				isSequence: false,
			},
		}

		for i := range testCases {
			tc := testCases[i]
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var bits bitArray
				for i := tc.startIndex; i < tc.numIndexes; i++ {
					err := bits.Set(i)
					require.NoError(t, err, "Set")
				}

				require.Equalf(t, tc.isSequence, bits.IsSequence(),
					"startIndex: %d, numIndexes: %d", tc.startIndex, tc.numIndexes,
				)
			})
		}
	})

	t.Run("Unset", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			var (
				expectedUnsetIdx = rand.Intn(32)
				expectedSetIdx   = rand.Intn(32) + 32
				bits             bitArray
			)

			err := bits.Set(expectedUnsetIdx)
			require.NoError(t, err)
			has, err := bits.Has(expectedUnsetIdx)
			require.NoError(t, err)
			require.True(t, has)

			err = bits.Set(expectedSetIdx)
			require.NoError(t, err)

			err = bits.Unset(expectedUnsetIdx)
			require.NoError(t, err)
			has, err = bits.Has(expectedUnsetIdx)
			require.NoError(t, err)
			require.False(t, has)

			has, err = bits.Has(expectedSetIdx)
			require.NoError(t, err)
			require.True(t, has)
		})

		t.Run("error: negative index", func(t *testing.T) {
			var (
				invalidIdx = -(rand.Intn(math.MaxInt32-1) + 1)
				bits       bitArray
			)

			err := bits.Unset(invalidIdx)
			assert.Error(t, err)
			assert.True(t, errorBitArrayInvalidIdx.Has(err), "errorBitArrayInvalidIdx class")
		})
	})
}
