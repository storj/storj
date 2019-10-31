// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/pkg/storj"
)

func TestNewList(t *testing.T) {
	log := zaptest.NewLogger(t)

	source, err := NewFixedSource("12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345")
	require.NoError(t, err)

	filter := NewFilter()

	cache, cacheDone := newTestCache(t, nil)
	defer cacheDone()

	for _, tt := range []struct {
		name    string
		log     *zap.Logger
		sources []Source
		filter  *Filter
		cache   *Cache
		err     string
	}{
		{
			name:    "missing logger",
			sources: []Source{source},
			filter:  filter,
			cache:   cache,
			err:     "trust: logger cannot be nil",
		},
		{
			name:   "missing sources",
			log:    log,
			filter: filter,
			cache:  cache,
			err:    "trust: at least one source must be configured",
		},
		{
			name:    "missing filter",
			log:     log,
			sources: []Source{source},
			cache:   cache,
			err:     "trust: filter cannot be nil",
		},
		{
			name:    "missing cache",
			log:     log,
			sources: []Source{source},
			filter:  filter,
			err:     "trust: cache cannot be nil",
		},
		{
			name:    "success",
			log:     log,
			sources: []Source{source},
			filter:  filter,
			cache:   cache,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			list, err := NewList(tt.log, tt.sources, tt.filter, tt.cache)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				require.Nil(t, list)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, list)
		})
	}
}

func TestListAgainstSpec(t *testing.T) {
	idReplacer := regexp.MustCompile(`^(\d)(@.*)$`)
	fixURL := func(s string) string {
		m := idReplacer.FindStringSubmatch(s)
		if m == nil {
			return s
		}
		return makeTestID(m[1][0]).String() + m[2]
	}

	makeNodeURL := func(s string) storj.NodeURL {
		u, err := storj.ParseNodeURL(fixURL(s))
		require.NoError(t, err)
		return u
	}

	makeSatelliteURL := func(s string) SatelliteURL {
		u, err := ParseSatelliteURL(fixURL(s))
		require.NoError(t, err)
		return u
	}

	makeEntry := func(s string, authoritative bool) Entry {
		return Entry{
			SatelliteURL:  makeSatelliteURL(s),
			Authoritative: authoritative,
		}
	}

	fileSource := &fakeSource{
		name:  "file:///path/to/some/trusted-satellites.txt",
		fixed: true,
		entries: []Entry{
			makeEntry("1@bar.test:7777", true),
		},
	}

	fooSource := &fakeSource{
		name:  "https://foo.test/trusted-satellites",
		fixed: false,
		entries: []Entry{
			makeEntry("2@f.foo.test:7777", true),
			makeEntry("2@buz.test:7777", false),
			makeEntry("2@qiz.test:7777", false),
			makeEntry("5@ohno.test:7777", false),
		},
	}

	barSource := &fakeSource{
		name:  "https://bar.test/trusted-satellites",
		fixed: false,
		entries: []Entry{
			makeEntry("3@f.foo.test:7777", false),
			makeEntry("3@bar.test:7777", true),
			makeEntry("3@baz.test:7777", false),
			makeEntry("3@buz.test:7777", false),
			makeEntry("3@quz.test:7777", false),
		},
	}

	bazSource := &fakeSource{
		name:  "https://baz.test/trusted-satellites",
		fixed: false,
		entries: []Entry{
			makeEntry("4@baz.test:7777", true),
			makeEntry("4@qiz.test:7777", false),
			makeEntry("4@subdomain.quz.test:7777", false),
		},
	}

	fixedSource := &fakeSource{
		name:  "0@f.foo.test:7777",
		fixed: true,
		entries: []Entry{
			makeEntry("0@f.foo.test:7777", true),
		},
	}

	filter := NewFilter()
	require.NoError(t, filter.Add("quz.test"))
	require.NoError(t, filter.Add(fixURL("2@qiz.test:7777")))
	require.NoError(t, filter.Add(fixURL("5@")))

	cache, cacheDone := newTestCache(t, nil)
	defer cacheDone()

	sources := []Source{
		fileSource,
		fooSource,
		barSource,
		bazSource,
		fixedSource,
	}

	log := zaptest.NewLogger(t)
	list, err := NewList(log, sources, filter, cache)
	require.NoError(t, err)

	urls, err := list.FetchURLs(context.Background())
	require.NoError(t, err)

	t.Logf("0@ = %s", makeTestID('0'))
	t.Logf("1@ = %s", makeTestID('1'))
	t.Logf("2@ = %s", makeTestID('2'))
	t.Logf("3@ = %s", makeTestID('3'))
	t.Logf("4@ = %s", makeTestID('4'))
	t.Logf("5@ = %s", makeTestID('5'))

	require.Equal(t, []storj.NodeURL{
		makeNodeURL("1@bar.test:7777"),
		makeNodeURL("2@f.foo.test:7777"),
		makeNodeURL("2@buz.test:7777"),
		makeNodeURL("4@baz.test:7777"),
		makeNodeURL("4@qiz.test:7777"),
	}, urls)
}

func TestListCacheInteraction(t *testing.T) {
	entry1 := Entry{
		SatelliteURL: SatelliteURL{
			Host: "host1",
			Port: 7777,
		},
	}
	url1 := entry1.SatelliteURL.NodeURL()

	entry2 := Entry{
		SatelliteURL: SatelliteURL{
			Host: "host2",
			Port: 7777,
		},
	}
	url2 := entry2.SatelliteURL.NodeURL()

	makeNormal := func(entries ...Entry) *fakeSource {
		return &fakeSource{
			name:    "normal",
			fixed:   false,
			entries: entries,
		}
	}

	makeFixed := func(entries ...Entry) *fakeSource {
		return &fakeSource{
			name:    "fixed",
			fixed:   true,
			entries: entries,
		}
	}

	badNormal := &fakeSource{
		name:  "normal",
		fixed: false,
		err:   errors.New("ohno"),
	}

	badFixed := &fakeSource{
		name:  "fixed",
		fixed: true,
		err:   errors.New("ohno"),
	}

	for _, tt := range []struct {
		name           string
		sources        []Source
		cacheBefore    map[string][]Entry
		cacheAfter     map[string][]Entry
		urls           []storj.NodeURL
		killCacheEarly bool
		err            string
	}{
		{
			name:    "entries are cached for normal sources",
			sources: []Source{makeNormal(entry1)},
			urls:    []storj.NodeURL{url1},
			cacheAfter: map[string][]Entry{
				"normal": {entry1},
			},
		},
		{
			name:       "entries are not cached for fixed sources",
			sources:    []Source{makeFixed(entry1)},
			urls:       []storj.NodeURL{url1},
			cacheAfter: map[string][]Entry{},
		},
		{
			name:    "entries are updated on success for normal sources",
			sources: []Source{makeNormal(entry2)},
			cacheBefore: map[string][]Entry{
				"normal": {entry1},
			},
			urls: []storj.NodeURL{url2},
			cacheAfter: map[string][]Entry{
				"normal": {entry2},
			},
		},
		{
			name:    "fetch fails if no cached entry on failure for normal source",
			sources: []Source{badNormal},
			err:     `trust: failed to fetch from source "normal": ohno`,
		},
		{
			name:    "cached entries are used on failure for normal sources",
			sources: []Source{badNormal},
			cacheBefore: map[string][]Entry{
				"normal": {entry1},
			},
			urls: []storj.NodeURL{url1},
			cacheAfter: map[string][]Entry{
				"normal": {entry1},
			},
		},
		{
			name:    "fetch fails on failure for fixed source",
			sources: []Source{badFixed},
			err:     `trust: failed to fetch from source "fixed": ohno`,
		},
		{
			name:           "failure to save cache is not fatal",
			sources:        []Source{makeNormal(entry1)},
			urls:           []storj.NodeURL{url1},
			killCacheEarly: true,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			cache, cacheDone := newTestCache(t, tt.cacheBefore)
			defer cacheDone()

			log := zaptest.NewLogger(t)
			filter := NewFilter()
			list, err := NewList(log, tt.sources, filter, cache)
			require.NoError(t, err)

			if tt.killCacheEarly {
				cacheDone()
			}

			urls, err := list.FetchURLs(context.Background())
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.urls, urls)

			if !tt.killCacheEarly {
				cacheAfter, err := loadCacheData(cache.Path())
				require.NoError(t, err)
				require.Equal(t, &cacheData{Entries: tt.cacheAfter}, cacheAfter)
			}
		})
	}
}

func newTestCache(t *testing.T, entries map[string][]Entry) (*Cache, func()) {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	done := func() {
		assert.NoError(t, os.RemoveAll(dir))
	}

	cachePath := filepath.Join(dir, "cache.json")

	err = saveCacheData(cachePath, &cacheData{
		Entries: entries,
	})
	if err != nil {
		done()
		require.NoError(t, err)
	}

	cache, err := LoadCache(cachePath)
	if err != nil {
		done()
		require.NoError(t, err)
	}

	return cache, done
}

type fakeSource struct {
	name    string
	fixed   bool
	entries []Entry
	err     error
}

func (s *fakeSource) String() string {
	return s.name
}

func (s *fakeSource) Fixed() bool {
	return s.fixed
}

func (s *fakeSource) FetchEntries(context.Context) ([]Entry, error) {
	return s.entries, s.err
}

func makeTestID(x byte) storj.NodeID {
	var id storj.NodeID
	copy(id[:], bytes.Repeat([]byte{x}, len(id)))
	return storj.NewVersionedID(id, storj.IDVersions[storj.V0])
}
