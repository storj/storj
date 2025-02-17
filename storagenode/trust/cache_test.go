// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/trust"
)

func TestCacheLoadCreatesDirectory(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cachePath := filepath.Join(ctx.Dir(), "sub", "cache.json")

	_, err := trust.LoadCache(cachePath)
	require.NoError(t, err)

	fi, err := os.Stat(filepath.Dir(cachePath))
	require.NoError(t, err, "cache directory should exist")
	require.True(t, fi.IsDir())

	_, err = os.Stat(cachePath)
	require.True(t, os.IsNotExist(err), "cache file should not exist")
}

func TestCacheLoadFailure(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cachePath := ctx.File("cache.json")

	// Use the directory itself as the path
	_, err := trust.LoadCache(ctx.Dir())
	assert.Error(t, err)

	// Load malformed JSON
	require.NoError(t, os.WriteFile(cachePath, []byte("BAD"), 0644))
	_, err = trust.LoadCache(cachePath)
	assert.EqualError(t, err, "trust: malformed cache: invalid character 'B' looking for beginning of value")
}

func TestCachePersistence(t *testing.T) {
	url1, err := trust.ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@foo.test:7777")
	require.NoError(t, err)

	url2, err := trust.ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@b.bar.test:7777")
	require.NoError(t, err)

	entry1 := trust.Entry{
		SatelliteURL:  url1,
		Authoritative: false,
	}

	entry2 := trust.Entry{
		SatelliteURL:  url2,
		Authoritative: true,
	}

	for _, tt := range []struct {
		name          string
		entriesBefore map[string][]trust.Entry
		lookup        []trust.Entry
		set           []trust.Entry
		save          bool
		entriesAfter  map[string][]trust.Entry
	}{
		{
			name: "new cache without save",
		},
		{
			name:         "new cache with save",
			save:         true,
			entriesAfter: map[string][]trust.Entry{},
		},
		{
			name: "set without save",
			set:  []trust.Entry{entry1, entry2},
			save: false,
		},
		{
			name: "set and save",
			set:  []trust.Entry{entry1, entry2},
			save: true,
			entriesAfter: map[string][]trust.Entry{
				"key": {entry1, entry2},
			},
		},
		{
			name: "replace without saving",
			entriesBefore: map[string][]trust.Entry{
				"key": {entry1},
			},
			lookup: []trust.Entry{entry1},
			set:    []trust.Entry{entry1, entry2},
			save:   false,
			entriesAfter: map[string][]trust.Entry{
				"key": {entry1},
			},
		},
		{
			name: "replace and save",
			entriesBefore: map[string][]trust.Entry{
				"key": {entry1},
			},
			lookup: []trust.Entry{entry1},
			set:    []trust.Entry{entry1, entry2},
			save:   true,
			entriesAfter: map[string][]trust.Entry{
				"key": {entry1, entry2},
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			cachePath := ctx.File("cache.json")

			if tt.entriesBefore != nil {
				require.NoError(t, trust.SaveCacheData(cachePath, &trust.CacheData{Entries: tt.entriesBefore}))
			}

			cache, err := trust.LoadCache(cachePath)
			require.NoError(t, err)

			entries, ok := cache.Lookup("key")
			if tt.lookup == nil {
				require.False(t, ok, "lookup should fail")
				require.Nil(t, entries, "failed lookup should produce nil entries slice")
			} else {
				require.True(t, ok, "lookup should succeed")
				require.Equal(t, tt.lookup, entries)
			}

			if tt.set != nil {
				cache.Set("key", tt.set)
			}

			if tt.save {
				require.NoError(t, cache.Save(context.Background()))
			}

			cacheAfter, err := trust.LoadCacheData(cachePath)
			if tt.entriesAfter == nil {
				require.Error(t, err)
				if !assert.True(t, errors.Is(err, fs.ErrNotExist), "cache file should not exist") {
					require.FailNow(t, "Expected cache file to not exist", "err=%w", err)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, &trust.CacheData{Entries: tt.entriesAfter}, cacheAfter)
			}
		})
	}

}

func TestCache_DeleteSatelliteEntry(t *testing.T) {
	url1, err := trust.ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@foo.test:7777")
	require.NoError(t, err)

	url2, err := trust.ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@b.bar.test:7777")
	require.NoError(t, err)

	entry1 := trust.Entry{
		SatelliteURL:  url1,
		Authoritative: false,
	}

	entry2 := trust.Entry{
		SatelliteURL:  url2,
		Authoritative: true,
	}

	entriesBefore := map[string][]trust.Entry{
		"key": {entry1, entry2},
	}

	expectedEntriesAfter := map[string][]trust.Entry{
		"key": {entry2},
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cachePath := ctx.File("cache.json")
	require.NoError(t, trust.SaveCacheData(cachePath, &trust.CacheData{Entries: entriesBefore}))

	cache, err := trust.LoadCache(cachePath)
	require.NoError(t, err)

	cache.DeleteSatelliteEntry(url1.ID)
	require.NoError(t, cache.Save(ctx))

	cacheAfter, err := trust.LoadCacheData(cachePath)
	require.NoError(t, err)
	require.Equal(t, &trust.CacheData{Entries: expectedEntriesAfter}, cacheAfter)
}
