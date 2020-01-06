// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
)

// List represents a dynamic trust list
type List struct {
	log     *zap.Logger
	sources Sources
	rules   Rules
	cache   *Cache
}

// NewList takes one or more sources, optional rules, and a cache and returns a new List.
func NewList(log *zap.Logger, sources []Source, rules Rules, cache *Cache) (*List, error) {
	// TODO: ideally we'd ensure there was at least one source configured since
	// it doesn't make sense to run a storage node that doesn't trust any
	// satellites, but unfortunately the check causes the backcompat tests to
	// fail.
	switch {
	case log == nil:
		return nil, Error.New("logger cannot be nil")
	case cache == nil:
		return nil, Error.New("cache cannot be nil")
	}
	return &List{
		log:     log,
		sources: sources,
		rules:   rules,
		cache:   cache,
	}, nil
}

// FetchURLs returns a list of Node URLS for trusted Satellites. It queries
// all of the configured sources for trust entries. Entries from non-fixed
// sources are cached. If entries cannot be retrieved from a source, a
// cached copy is used, if available. Otherwise, if there are no cached
// entries available, the call will fail. The URLS are filtered before being
// returned.
func (list *List) FetchURLs(ctx context.Context) ([]storj.NodeURL, error) {
	candidates, err := list.fetchEntries(ctx)
	if err != nil {
		return nil, err
	}

	byAddress := make(map[string]int)
	entries := make([]Entry, 0, len(candidates))
	for _, entry := range candidates {
		if !list.rules.IsTrusted(entry.SatelliteURL) {
			continue
		}
		previousIdx, ok := byAddress[entry.SatelliteURL.Address()]
		if ok {
			previous := entries[previousIdx]
			// An entry with the same address has already been aggregated.
			// If the entry is authoritative and the the previous entry was not
			// then replace the previous entry, otherwise ignore.
			if entry.Authoritative && !previous.Authoritative {
				entries[previousIdx] = entry
			}
			continue
		}

		byAddress[entry.SatelliteURL.Address()] = len(entries)
		entries = append(entries, entry)
	}

	var urls []storj.NodeURL
	for _, entry := range entries {
		urls = append(urls, entry.SatelliteURL.NodeURL())
	}
	return urls, nil
}

func (list *List) fetchEntries(ctx context.Context) (_ []Entry, err error) {
	defer mon.Task()(&ctx)(&err)

	var allEntries []Entry
	for _, source := range list.sources {
		sourceLog := list.log.With(zap.String("source", source.String()))

		entries, err := source.FetchEntries(ctx)
		if err != nil {
			var ok bool
			entries, ok = list.lookupCache(source)
			if !ok {
				sourceLog.Error("Failed to fetch URLs from source", zap.Error(err))
				return nil, Error.New("failed to fetch from source %q: %v", source.String(), err)
			}
			sourceLog.Warn("Failed to fetch URLs from source; used cache", zap.Error(err))
		} else {
			sourceLog.Debug("Fetched URLs from source; updating cache", zap.Int("count", len(entries)))
			list.updateCache(source, entries)
		}

		allEntries = append(allEntries, entries...)
	}

	if err := list.saveCache(ctx); err != nil {
		list.log.Warn("Unable to save list cache", zap.Error(err))
	}
	return allEntries, nil
}

func (list *List) lookupCache(source Source) ([]Entry, bool) {
	// Static sources are not cached
	if source.Static() {
		return nil, false
	}
	return list.cache.Lookup(source.String())
}

func (list *List) updateCache(source Source, entries []Entry) {
	// Static sources are not cached
	if source.Static() {
		return
	}
	list.cache.Set(source.String(), entries)
}

func (list *List) saveCache(ctx context.Context) error {
	return list.cache.Save(ctx)
}
