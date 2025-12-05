// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/zeebo/assert"
	"github.com/zeebo/mwc"
)

func TestFlatMap_InsertAndLookup(t *testing.T) {
	var val [4]byte
	var keys []shortKey

	m := newFlatMap(make([]byte, flatMapSize(1000)))

	for i := 0; i < 1000; i++ {
		binary.LittleEndian.PutUint32(val[:], uint32(i))
		key := shortKeyFrom(newKey())
		keys = append(keys, key)
		m.find(key).set(val)
	}

	for i := 0; i < 1000; i++ {
		binary.LittleEndian.PutUint32(val[:], uint32(i))
		assert.Equal(t, m.find(keys[i]).Value(), val)
	}
}

func TestFlatMap_Update(t *testing.T) {
	m := newFlatMap(make([]byte, flatMapSize(100)))

	keys := make(map[shortKey]struct{})
	uniqueKey := func() (s shortKey) {
		for {
			s = shortKeyFrom(newKey())
			if _, exists := keys[s]; !exists {
				keys[s] = struct{}{}
				return s
			}
		}
	}

	// Insert initial values
	for i := 0; i < 50; i++ {
		key := uniqueKey()

		// Initial value
		op := m.find(key)
		op.set([4]byte{0: 1})

		// Verify it was inserted correctly
		assert.Equal(t, m.find(key).Value(), [4]byte{0: 1})
	}

	// Update the values
	for key := range keys {
		// find the key
		op := m.find(key)
		assert.That(t, op.Exists())
		assert.Equal(t, op.Value(), [4]byte{0: 1})

		// update the value
		op.set([4]byte{0: 2})

		// Verify the update worked
		assert.Equal(t, m.find(key).Value(), [4]byte{0: 2})
	}
}

func TestFlatMap_LookupMissingKeys(t *testing.T) {
	m := newFlatMap(make([]byte, flatMapSize(100)))

	keys := make(map[shortKey]struct{})
	uniqueKey := func() (s shortKey) {
		for {
			s = shortKeyFrom(newKey())
			if _, exists := keys[s]; !exists {
				keys[s] = struct{}{}
				return s
			}
		}
	}

	// Insert some keys
	for i := 0; i < 50; i++ {
		m.find(uniqueKey()).set([4]byte{0: byte(i)})
	}

	// Verify that looking up missing keys works as expected
	for i := 0; i < 50; i++ {
		op := m.find(uniqueKey())
		assert.That(t, !op.Exists())
		assert.That(t, op.Valid())
	}
}

func TestFlatMap_FullMap(t *testing.T) {
	m := newFlatMap(make([]byte, flatMapSize(8)))

	// Fill the map
	for i := 0; i < 8; i++ {
		op := m.find(shortKey{0: byte(i)})
		assert.That(t, op.Valid())
		op.set([4]byte{0: byte(i)})
	}

	// Verify all entries can be found
	for i := 0; i < 8; i++ {
		op := m.find(shortKey{0: byte(i)})
		assert.That(t, op.Exists())
		assert.Equal(t, op.Value(), [4]byte{0: byte(i)})
	}

	// This should fail because the map is full
	op := m.find(shortKey{0: 8})
	assert.That(t, !op.Valid())
}

func TestFlatMap_EdgeCases(t *testing.T) {
	m := newFlatMap(make([]byte, flatMapSize(100)))

	// Test with zero value
	zeroKey := shortKey{}
	zeroVal := [4]byte{}
	op := m.find(zeroKey)
	op.set(zeroVal)

	// Verify zero key/value can be found
	lookupOp := m.find(zeroKey)
	assert.That(t, lookupOp.Exists())
	assert.Equal(t, lookupOp.Value(), zeroVal)

	// Test with repeated inserts of the same key
	repeatedKey := [5]byte{5, 4, 3, 2, 1}
	values := [][4]byte{
		{1, 0, 0, 0},
		{1, 2, 0, 0},
		{1, 2, 3, 0},
		{1, 2, 3, 4},
	}

	for _, val := range values {
		op := m.find(repeatedKey)
		assert.That(t, op.Valid())
		op.set(val)

		// Verify the updated value
		lookup := m.find(repeatedKey)
		assert.That(t, lookup.Exists())
		assert.Equal(t, lookup.Value(), val)
	}
}

func TestFlatMap_ZeroSizedMap(t *testing.T) {
	// Create a zero-sized map
	m := newFlatMap(make([]byte, 0))

	// Test with any key
	key := [5]byte{1, 2, 3, 4, 5}

	// The find operation should be invalid
	op := m.find(key)
	assert.That(t, !op.Valid())
}

func TestFlatMap_RandomOperations(t *testing.T) {
	m := newFlatMap(make([]byte, flatMapSize(1000)))
	rng := mwc.Rand()
	expect := make(map[shortKey][4]byte)

	for i := 0; i < 5000; i++ {
		key := shortKeyFrom(newKey())

		switch rng.Uint64n(3) {
		case 0, 1: // insert or update
			var val [4]byte
			_, _ = rng.Read(val[:])

			op := m.find(key)

			expectVal, exists := expect[key]
			assert.Equal(t, op.Exists(), exists)
			if exists {
				assert.Equal(t, op.Value(), expectVal)
			}

			if op.Valid() {
				op.set(val)
				expect[key] = val
			}

		case 2: // lookup
			op := m.find(key)

			expectVal, exists := expect[key]
			assert.Equal(t, op.Exists(), exists)
			if exists {
				assert.Equal(t, op.Value(), expectVal)
			}
		}
	}
}

//
// benchmarks
//

func BenchmarkMaps(b *testing.B) {
	benchmarkLRecs(b, "flat", func(b *testing.B, u uint64) {
		b.Run("Insert", func(b *testing.B) {
			nrec := int(float64(uint64(1)<<u) * 0.85)
			rng := mwc.Rand()
			now := time.Now()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				m := newFlatMap(make([]byte, flatMapSize(1<<u)))
				b.StartTimer()

				var key shortKey
				for j := 0; j < nrec; j++ {
					_, _ = rng.Read(key[:])
					m.find(key).set([4]byte{0xde, 0xad, 0xbe, 0xef})
				}
			}

			b.ReportMetric(float64(nrec*b.N)/time.Since(now).Seconds(), "keys/sec")
		})
	})

	benchmarkLRecs(b, "go", func(b *testing.B, u uint64) {
		b.Run("Insert", func(b *testing.B) {
			nrec := int(float64(uint64(1)<<u) * 0.85)
			rng := mwc.Rand()
			now := time.Now()

			b.ReportAllocs()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				m := make(map[shortKey][4]byte, nrec)
				b.StartTimer()

				var key shortKey
				for j := 0; j < nrec; j++ {
					_, _ = rng.Read(key[:])
					m[key] = [4]byte{0xde, 0xad, 0xbe, 0xef}
				}
			}

			b.ReportMetric(float64(nrec*b.N)/time.Since(now).Seconds(), "keys/sec")
		})
	})
}
