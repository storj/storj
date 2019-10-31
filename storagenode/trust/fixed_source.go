// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"context"
)

// FixedSource is a trust source that returns an explicitly trusted URL
type FixedSource struct {
	url SatelliteURL
}

// NewFixedSource takes an explicitly trusted URL and returns a new FixedSource.
func NewFixedSource(satelliteURL string) (*FixedSource, error) {
	url, err := ParseSatelliteURL(satelliteURL)
	if err != nil {
		return nil, err
	}
	return &FixedSource{url: url}, nil
}

// String implements the Source interface and returns the fixed trusted URL
func (source *FixedSource) String() string {
	return source.url.String()
}

// Fixed implements the Source interface. It returns true.
func (source *FixedSource) Fixed() bool { return true }

// FetchEntries returns a trust entry for the explicitly trusted Satellite URL.
// The entry is authoritative.
func (source *FixedSource) FetchEntries(ctx context.Context) ([]Entry, error) {
	return []Entry{
		{SatelliteURL: source.url, Authoritative: true},
	}, nil
}
