// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
)

func TestCacheLoadCreatesDirectory(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer func() { assert.NoError(t, os.RemoveAll(dir)) }()

	cachePath := filepath.Join(dir, "sub", "cache.json")

	_, err = LoadCache(cachePath)
	require.NoError(t, err)

	fi, err := os.Stat(filepath.Dir(cachePath))
	require.NoError(t, err, "cache directory should exist")
	require.True(t, fi.IsDir())

	_, err = os.Stat(cachePath)
	require.True(t, os.IsNotExist(err), "cache file should not exist")
}

func TestCacheLoadFailure(t *testing.T) {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer func() { assert.NoError(t, os.RemoveAll(dir)) }()

	cachePath := filepath.Join(dir, "cache.json")

	// Use the directory itself as the path
	_, err = LoadCache(dir)
	assert.EqualError(t, err, fmt.Sprintf("trust: read %s: is a directory", dir))

	// Load malformed JSON
	require.NoError(t, ioutil.WriteFile(cachePath, []byte("BAD"), 0644))
	_, err = LoadCache(cachePath)
	assert.EqualError(t, err, "trust: malformed cache: invalid character 'B' looking for beginning of value")
}

func TestCachePersistence(t *testing.T) {
	url1, err := ParseSatelliteURL("121RTSDpyNZVcEU84Ticf2L1ntiuUimbWgfATz21tuvgk3vzoA6@foo.test:7777")
	require.NoError(t, err)

	url2, err := ParseSatelliteURL("12L9ZFwhzVpuEKMUNUqkaTLGzwY9G24tbiigLiXpmZWKwmcNDDs@b.bar.test:7777")
	require.NoError(t, err)

	entry1 := Entry{
		SatelliteURL:  url1,
		Authoritative: false,
	}

	entry2 := Entry{
		SatelliteURL:  url2,
		Authoritative: true,
	}

	for _, tt := range []struct {
		name          string
		entriesBefore map[string][]Entry
		lookup        []Entry
		set           []Entry
		save          bool
		entriesAfter  map[string][]Entry
	}{
		{
			name: "new cache without save",
		},
		{
			name:         "new cache with save",
			save:         true,
			entriesAfter: map[string][]Entry{},
		},
		{
			name: "set without save",
			set:  []Entry{entry1, entry2},
			save: false,
		},
		{
			name: "set and save",
			set:  []Entry{entry1, entry2},
			save: true,
			entriesAfter: map[string][]Entry{
				"key": {entry1, entry2},
			},
		},
		{
			name: "replace without saving",
			entriesBefore: map[string][]Entry{
				"key": {entry1},
			},
			lookup: []Entry{entry1},
			set:    []Entry{entry1, entry2},
			save:   false,
			entriesAfter: map[string][]Entry{
				"key": {entry1},
			},
		},
		{
			name: "replace and save",
			entriesBefore: map[string][]Entry{
				"key": {entry1},
			},
			lookup: []Entry{entry1},
			set:    []Entry{entry1, entry2},
			save:   true,
			entriesAfter: map[string][]Entry{
				"key": {entry1, entry2},
			},
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "")
			require.NoError(t, err)
			defer func() { assert.NoError(t, os.RemoveAll(dir)) }()

			cachePath := filepath.Join(dir, "cache.json")

			if tt.entriesBefore != nil {
				require.NoError(t, saveCacheData(cachePath, &cacheData{Entries: tt.entriesBefore}))
			}

			cache, err := LoadCache(cachePath)
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

			cacheAfter, err := loadCacheData(cachePath)
			if tt.entriesAfter == nil {
				require.Error(t, err)
				if !assert.True(t, os.IsNotExist(errs.Unwrap(err)), "cache file should not exist") {
					require.FailNow(t, "Expected cache file to not exist", "err=%v", err)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, &cacheData{Entries: tt.entriesAfter}, cacheAfter)
			}
		})
	}

}
