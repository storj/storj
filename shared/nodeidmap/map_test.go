// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeidmap_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/shared/nodeidmap"
)

// testid constructs a node id for testing, setting the first byte as the prefix and rest as the suffix.
func testid(x string) (r storj.NodeID) {
	r[0] = x[0]
	copy(r[4:], x[1:])
	return r
}

func TestMap(t *testing.T) {
	xs := nodeidmap.Make[int]()

	// Range is implicitly tested via .AsMap

	// Store

	xs.Store(testid("123"), 123)
	xs.Store(testid("145"), 145)
	xs.Store(testid("223"), 223)
	xs.Store(testid("245"), 245)

	require.Equal(t, map[storj.NodeID]int{
		testid("123"): 123,
		testid("145"): 145,
		testid("223"): 223,
		testid("245"): 245,
	}, xs.AsMap())

	// Load

	got, ok := xs.Load(testid("123"))
	require.True(t, ok)
	require.Equal(t, 123, got)

	got, ok = xs.Load(testid("245"))
	require.True(t, ok)
	require.Equal(t, 245, got)

	got, ok = xs.Load(testid("199"))
	require.False(t, ok)
	require.Equal(t, 0, got)

	got, ok = xs.Load(testid("299"))
	require.False(t, ok)
	require.Equal(t, 0, got)

	// Modify

	xs.Modify(testid("145"), func(old int, ok bool) int {
		require.True(t, ok)
		require.Equal(t, 145, old)
		return 1450
	})

	xs.Modify(testid("267"), func(old int, ok bool) int {
		require.False(t, ok)
		require.Equal(t, 0, old)
		return 267
	})

	require.Equal(t, map[storj.NodeID]int{
		testid("123"): 123,
		testid("145"): 1450,
		testid("223"): 223,
		testid("245"): 245,
		testid("267"): 267,
	}, xs.AsMap())

	// Clone

	require.Equal(t, xs.AsMap(), xs.Clone().AsMap())
}

func TestMap_Add(t *testing.T) {
	a := nodeidmap.Make[int]()
	b := nodeidmap.Make[int]()

	// check that order doesn't matter when combining
	a.Store(testid("12"), 0x12_00)
	a.Store(testid("13"), 0x13_00)
	b.Store(testid("13"), 0x00_13)
	b.Store(testid("12"), 0x00_12)

	// check that new entries in linked lists are added
	a.Store(testid("24"), 0x24_00)
	b.Store(testid("25"), 0x00_25)

	// check that new prefixes are preserved and added
	a.Store(testid("36"), 0x36_00)
	b.Store(testid("47"), 0x00_47)
	b.Store(testid("48"), 0x00_48)

	a.Add(b, func(old, new int) int { return old + new })

	require.Equal(t, map[storj.NodeID]int{
		testid("12"): 0x12_12,
		testid("13"): 0x13_13,
		testid("24"): 0x24_00,
		testid("25"): 0x00_25,
		testid("36"): 0x36_00,
		testid("47"): 0x00_47,
		testid("48"): 0x00_48,
	}, a.AsMap())
}

func BenchmarkLoad(b *testing.B) {
	type Entry struct {
		ID    storj.NodeID
		Value int32
	}

	var entries []Entry
	for i := 0; i < 20000; i++ {
		entries = append(entries, Entry{
			ID:    testrand.NodeID(),
			Value: int32(i),
		})
	}

	testindexes := []int{}
	for i := 0; i < 100; i++ {
		testindexes = append(testindexes, testrand.Intn(len(entries)))
	}

	m := nodeidmap.Make[int32]()
	g := map[storj.NodeID]int32{}
	for _, e := range entries {
		m.Store(e.ID, e.Value)
		g[e.ID] = e.Value
	}

	b.Run("Map", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, ix := range testindexes {
				entry := &entries[ix]
				v, ok := m.Load(entry.ID)
				if !ok || v != entry.Value {
					b.Fatal("wrong result")
				}
			}
		}
	})

	b.Run("Go", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for _, ix := range testindexes {
				entry := &entries[ix]
				v, ok := g[entry.ID]
				if !ok || v != entry.Value {
					b.Fatal("wrong result")
				}
			}
		}
	})
}
