// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/common/fpath"
)

// Cache caches source information about trusted satellites
type Cache struct {
	path string
	data *CacheData
}

// LoadCache loads a cache from a file on disk. If the file is not present, the
// cache is still loaded.  If the file cannot be read for any other reason, the
// function will return an error. LoadCache ensures the containing directory
// exists.
func LoadCache(path string) (*Cache, error) {
	if path == "" {
		return nil, Error.New("cache path cannot be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return nil, Error.New("unable to make cache parent directory: %v", err)
	}

	data, err := LoadCacheData(path)
	switch {
	case err == nil:
	case errs.IsFunc(err, os.IsNotExist):
		data = NewCacheData()
	default:
		return nil, err
	}

	return &Cache{
		path: path,
		data: data,
	}, nil
}

// Path returns the path on disk to the file containing the cache
func (cache *Cache) Path() string {
	return cache.path
}

// Lookup takes a cache key and returns entries associated with that key. If
// the key is unset in the cache, false is returned for ok. Otherwise the
// entries are returned with ok returned as true.
func (cache *Cache) Lookup(key string) (entries []Entry, ok bool) {
	entries, ok = cache.data.Entries[key]
	return entries, ok
}

// Set sets the entries in the cache for the provided key
func (cache *Cache) Set(key string, entries []Entry) {
	cache.data.Entries[key] = entries
}

// Save persists the cache to disk
func (cache *Cache) Save(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	return SaveCacheData(cache.path, cache.data)
}

// CacheData represents the data stored in the cache
type CacheData struct {
	Entries map[string][]Entry `json:"entries"`
}

// NewCacheData returns an new CacheData
func NewCacheData() *CacheData {
	return &CacheData{
		Entries: make(map[string][]Entry),
	}
}

// LoadCacheData loads the cache data from the given path
func LoadCacheData(path string) (*CacheData, error) {
	dataBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	data := NewCacheData()
	if err := json.Unmarshal(dataBytes, data); err != nil {
		return nil, Error.New("malformed cache: %v", err)
	}
	// Ensure the entries map is always non-nil on load
	if data.Entries == nil {
		data.Entries = map[string][]Entry{}
	}
	return data, nil
}

// SaveCacheData persists the cache data to the given path
func SaveCacheData(path string, data *CacheData) error {
	// Ensure the entries map is always non-nil on save
	if data.Entries == nil {
		data.Entries = map[string][]Entry{}
	}
	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return Error.Wrap(err)
	}
	return fpath.AtomicWriteFile(path, dataBytes, 0644)
}
