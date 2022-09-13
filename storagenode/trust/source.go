// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
	"regexp"

	"github.com/zeebo/errs"
)

// Entry represents a trust entry.
type Entry struct {
	// SatelliteURL is the URL of the satellite
	SatelliteURL SatelliteURL

	// Authoritative indicates whether this entry came from an authoritative
	// source. This impacts how URLS are aggregated.
	Authoritative bool `json:"authoritative"`
}

// Source is a trust source for trusted Satellites.
type Source interface {
	// String is the string representation of the source. It is used as a key
	// into the cache.
	String() string

	// Static returns true if the source is static. Static sources are not cached.
	Static() bool

	// FetchEntries returns the list of trust entries from the source.
	FetchEntries(context.Context) ([]Entry, error)
}

// NewSource takes a configuration string returns a Source for that string.
func NewSource(config string) (Source, error) {
	schema, ok := isReserved(config)
	if ok {
		switch schema {
		case "http", "https":
			return NewHTTPSource(config)
		case "storj":
			return NewStaticURLSource(config)
		default:
			return nil, errs.New("unsupported schema %q", schema)
		}
	}

	if isProbablySatelliteURL(config) {
		return NewStaticURLSource(config)
	}

	return NewFileSource(config), nil
}

var reReserved = regexp.MustCompile(`^([a-zA-Z]{2,})://`)

// isReserved returns the true if the string is within the reserved namespace
// for trust sources, i.e. things that look like a URI scheme. Single letter
// schemes are not in the reserved namespace since those collide with paths
// starting with Windows drive letters.
func isReserved(s string) (schema string, ok bool) {
	m := reReserved.FindStringSubmatch(s)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// reProbablySatelliteURL matches config strings that are (intended, but
// possibly misconfigured) satellite URLs, like the following:
//
//   - @
//   - id@
//   - host:9999
//   - id@host:9999
var reProbablySatelliteURL = regexp.MustCompile(`@|(^[^/\\]{2,}:\d+$)`)

func isProbablySatelliteURL(s string) bool {
	// Painful esoteric paths to consider if you want to change the regex. None
	// of the paths below should be parsed as satellite URLs, which the
	// exception of the last one, which would fail since it does not contain an
	// ID portion but would fail with good diagnostics.
	// 1. http://basic:auth@example.com
	// 2. c:/windows
	// 3. c:\\windows
	// 3. \\?\c:\\windows
	// 4. probably other nightmarish windows paths
	// 5. /posix/paths:are/terrible
	// 6. posix.paths.are.really.terrible:7777
	return reProbablySatelliteURL.MatchString(s)
}
