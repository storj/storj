// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/storagenode/trust"
)

func TestNewList(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	cache := newTestCache(t, ctx.Dir(), nil)

	for _, tt := range []struct {
		name  string
		log   *zap.Logger
		cache *trust.Cache
		err   string
	}{
		{
			name:  "missing logger",
			cache: cache,
			err:   "trust: logger cannot be nil",
		},
		{
			name: "missing cache",
			log:  log,
			err:  "trust: cache cannot be nil",
		},
		{
			name:  "success",
			log:   log,
			cache: cache,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			list, err := trust.NewList(tt.log, nil, nil, tt.cache)
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
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

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

	makeSatelliteURL := func(s string) trust.SatelliteURL {
		u, err := trust.ParseSatelliteURL(fixURL(s))
		require.NoError(t, err)
		return u
	}

	makeEntry := func(s string, authoritative bool) trust.Entry {
		return trust.Entry{
			SatelliteURL:  makeSatelliteURL(s),
			Authoritative: authoritative,
		}
	}

	fileSource := &fakeSource{
		name:   "file:///path/to/some/trusted-satellites.txt",
		static: true,
		entries: []trust.Entry{
			makeEntry("1@bar.test:7777", true),
		},
	}

	fooSource := &fakeSource{
		name:   "https://foo.test/trusted-satellites",
		static: false,
		entries: []trust.Entry{
			makeEntry("2@f.foo.test:7777", true),
			makeEntry("2@buz.test:7777", false),
			makeEntry("2@qiz.test:7777", false),
			makeEntry("5@ohno.test:7777", false),
		},
	}

	barSource := &fakeSource{
		name:   "https://bar.test/trusted-satellites",
		static: false,
		entries: []trust.Entry{
			makeEntry("3@f.foo.test:7777", false),
			makeEntry("3@bar.test:7777", true),
			makeEntry("3@baz.test:7777", false),
			makeEntry("3@buz.test:7777", false),
			makeEntry("3@quz.test:7777", false),
		},
	}

	bazSource := &fakeSource{
		name:   "https://baz.test/trusted-satellites",
		static: false,
		entries: []trust.Entry{
			makeEntry("4@baz.test:7777", true),
			makeEntry("4@qiz.test:7777", false),
			makeEntry("4@subdomain.quz.test:7777", false),
		},
	}

	fixedSource := &fakeSource{
		name:   "0@f.foo.test:7777",
		static: true,
		entries: []trust.Entry{
			makeEntry("0@f.foo.test:7777", true),
		},
	}

	rules := trust.Rules{
		trust.NewHostExcluder("quz.test"),
		trust.NewURLExcluder(makeSatelliteURL("2@qiz.test:7777")),
		trust.NewIDExcluder(makeTestID('5')),
	}

	cache := newTestCache(t, ctx.Dir(), nil)

	sources := []trust.Source{
		fileSource,
		fooSource,
		barSource,
		bazSource,
		fixedSource,
	}

	log := zaptest.NewLogger(t)
	list, err := trust.NewList(log, sources, rules, cache)
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
	entry1 := trust.Entry{
		SatelliteURL: trust.SatelliteURL{
			Host: "host1",
			Port: 7777,
		},
	}
	url1 := entry1.SatelliteURL.NodeURL()

	entry2 := trust.Entry{
		SatelliteURL: trust.SatelliteURL{
			Host: "host2",
			Port: 7777,
		},
	}
	url2 := entry2.SatelliteURL.NodeURL()

	makeNormal := func(entries ...trust.Entry) *fakeSource {
		return &fakeSource{
			name:    "normal",
			static:  false,
			entries: entries,
		}
	}

	makeFixed := func(entries ...trust.Entry) *fakeSource {
		return &fakeSource{
			name:    "static",
			static:  true,
			entries: entries,
		}
	}

	badNormal := &fakeSource{
		name:   "normal",
		static: false,
		err:    errors.New("ohno"),
	}

	badFixed := &fakeSource{
		name:   "static",
		static: true,
		err:    errors.New("ohno"),
	}

	for _, tt := range []struct {
		name           string
		sources        []trust.Source
		cacheBefore    map[string][]trust.Entry
		cacheAfter     map[string][]trust.Entry
		urls           []storj.NodeURL
		killCacheEarly bool
		err            string
	}{
		{
			name:    "entries are cached for normal sources",
			sources: []trust.Source{makeNormal(entry1)},
			urls:    []storj.NodeURL{url1},
			cacheAfter: map[string][]trust.Entry{
				"normal": {entry1},
			},
		},
		{
			name:       "entries are not cached for static sources",
			sources:    []trust.Source{makeFixed(entry1)},
			urls:       []storj.NodeURL{url1},
			cacheAfter: map[string][]trust.Entry{},
		},
		{
			name:    "entries are updated on success for normal sources",
			sources: []trust.Source{makeNormal(entry2)},
			cacheBefore: map[string][]trust.Entry{
				"normal": {entry1},
			},
			urls: []storj.NodeURL{url2},
			cacheAfter: map[string][]trust.Entry{
				"normal": {entry2},
			},
		},
		{
			name:    "fetch fails if no cached entry on failure for normal source",
			sources: []trust.Source{badNormal},
			err:     `trust: failed to fetch from source "normal": ohno`,
		},
		{
			name:    "cached entries are used on failure for normal sources",
			sources: []trust.Source{badNormal},
			cacheBefore: map[string][]trust.Entry{
				"normal": {entry1},
			},
			urls: []storj.NodeURL{url1},
			cacheAfter: map[string][]trust.Entry{
				"normal": {entry1},
			},
		},
		{
			name:    "fetch fails on failure for static source",
			sources: []trust.Source{badFixed},
			err:     `trust: failed to fetch from source "static": ohno`,
		},
		{
			name:           "failure to save cache is not fatal",
			sources:        []trust.Source{makeNormal(entry1)},
			urls:           []storj.NodeURL{url1},
			killCacheEarly: true,
		},
	} {
		tt := tt // quiet linting
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			cache := newTestCache(t, ctx.Dir(), tt.cacheBefore)

			log := zaptest.NewLogger(t)
			list, err := trust.NewList(log, tt.sources, nil, cache)
			require.NoError(t, err)

			if tt.killCacheEarly {
				require.NoError(t, os.Remove(cache.Path()))
			}

			urls, err := list.FetchURLs(context.Background())
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.urls, urls)

			if !tt.killCacheEarly {
				cacheAfter, err := trust.LoadCacheData(cache.Path())
				require.NoError(t, err)
				require.Equal(t, &trust.CacheData{Entries: tt.cacheAfter}, cacheAfter)
			}
		})
	}
}

func newTestCache(t *testing.T, dir string, entries map[string][]trust.Entry) *trust.Cache {
	cachePath := filepath.Join(dir, "cache.json")

	err := trust.SaveCacheData(cachePath, &trust.CacheData{
		Entries: entries,
	})
	require.NoError(t, err)

	cache, err := trust.LoadCache(cachePath)
	require.NoError(t, err)

	return cache
}

type fakeSource struct {
	name    string
	static  bool
	entries []trust.Entry
	err     error
}

func (s *fakeSource) String() string {
	return s.name
}

func (s *fakeSource) Static() bool {
	return s.static
}

func (s *fakeSource) FetchEntries(context.Context) ([]trust.Entry, error) {
	return s.entries, s.err
}

func makeTestID(x byte) storj.NodeID {
	var id storj.NodeID
	copy(id[:], bytes.Repeat([]byte{x}, len(id)))
	return storj.NewVersionedID(id, storj.IDVersions[storj.V0])
}
