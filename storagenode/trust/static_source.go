// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"

	"github.com/zeebo/errs"
)

var (
	// ErrStaticSource is an error class for static source errors.
	ErrStaticSource = errs.Class("static source")
)

// StaticURLSource is a trust source that returns an explicitly trusted URL.
type StaticURLSource struct {
	URL SatelliteURL
}

// NewStaticURLSource takes an explicitly trusted URL and returns a new StaticURLSource.
func NewStaticURLSource(satelliteURL string) (*StaticURLSource, error) {
	url, err := ParseSatelliteURL(satelliteURL)
	if err != nil {
		return nil, ErrStaticSource.Wrap(err)
	}
	return &StaticURLSource{URL: url}, nil
}

// String implements the Source interface and returns the static trusted URL.
func (source *StaticURLSource) String() string {
	return source.URL.String()
}

// Static implements the Source interface. It returns true.
func (source *StaticURLSource) Static() bool { return true }

// FetchEntries returns a trust entry for the explicitly trusted Satellite URL.
// The entry is authoritative.
func (source *StaticURLSource) FetchEntries(ctx context.Context) ([]Entry, error) {
	return []Entry{
		{SatelliteURL: source.URL, Authoritative: true},
	}, nil
}
