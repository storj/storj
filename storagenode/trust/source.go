// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"strings"
)

// Entry represents a trust entry
type Entry struct {
	// SatelliteURL is the URL of the satellite
	SatelliteURL SatelliteURL

	// Authoritative indicates whether this entry came from an authoritative
	// source. This impacts how URLS are aggregated.
	Authoritative bool `json:"authoritative"`
}

// Source is a trust source for trusted Satellites
type Source interface {
	// String is the string representation of the source. It is used as a key
	// into the cache.
	String() string

	// Fixed returns true if the source is fixed. Fixed sources are not cached.
	Fixed() bool

	// FetchEntries returns the list of trust entries from the source.
	FetchEntries(context.Context) ([]Entry, error)
}

// NewSource takes a configuration string returns a Source for that string.
// Supported strings are 1) a file:// URL, 2) an http:// URL, 3) an https:// URL
// and 4) a Satellite URL.
func NewSource(config string) (Source, error) {
	switch {
	case strings.HasPrefix(config, "file://"):
		return NewFileSource(config)
	case strings.HasPrefix(config, "http://"), strings.HasPrefix(config, "https://"):
		return NewHTTPSource(config)
	default:
		return NewFixedSource(config)
	}
}
