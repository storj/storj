// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"github.com/zeebo/errs"
)

var (
	// ErrStaticSource is an error class for static source errors
	ErrStaticSource = errs.Class("static source")
)

// StaticSource is a trust source that returns an explicitly trusted URL
type StaticSource struct {
	url SatelliteURL
}

// NewStaticSource takes an explicitly trusted URL and returns a new StaticSource.
func NewStaticSource(satelliteURL string) (*StaticSource, error) {
	url, err := ParseSatelliteURL(satelliteURL)
	if err != nil {
		return nil, ErrStaticSource.Wrap(err)
	}
	return &StaticSource{url: url}, nil
}

// String implements the Source interface and returns the static trusted URL
func (source *StaticSource) String() string {
	return source.url.String()
}

// Static implements the Source interface. It returns true.
func (source *StaticSource) Static() bool { return true }

// FetchEntries returns a trust entry for the explicitly trusted Satellite URL.
// The entry is authoritative.
func (source *StaticSource) FetchEntries(ctx context.Context) ([]Entry, error) {
	return []Entry{
		{SatelliteURL: source.url, Authoritative: true},
	}, nil
}
