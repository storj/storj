// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"net"
	"strconv"

	"storj.io/storj/pkg/storj"
)

// Filter filters untrusted URLs
type Filter struct {
	urls  map[SatelliteURL]struct{}
	hosts *HostSet
	ids   map[storj.NodeID]struct{}
}

// NewFilter creates a new filter. By default the filter passes everything.
func NewFilter() *Filter {
	return &Filter{
		urls:  make(map[SatelliteURL]struct{}),
		hosts: NewHostSet(),
		ids:   make(map[storj.NodeID]struct{}),
	}
}

// Passes takes a Satellite URL and returns true if the URL passes the filter
// (i.e. is trusted) or false otherwise.
func (filter *Filter) Passes(url SatelliteURL) bool {
	if _, ok := filter.urls[url]; ok {
		return false
	}
	if _, ok := filter.ids[url.ID]; ok {
		return false
	}
	if filter.hosts.Includes(url.Host) {
		return false
	}
	return true
}

// Add takes a configuration string and modifies the filter accordingly. Accepted
// forms are 1) a Satellite ID followed by '@', 2) a hostname or IP address, 3)
// a full Satellite URL.
func (filter *Filter) Add(config string) error {
	url, err := parseFilterConfig(config)
	if err != nil {
		return err
	}

	switch {
	case url.Host == "":
		filter.ids[url.ID] = struct{}{}
	case url.ID.IsZero():
		filter.hosts.Add(url.Host)
	default:
		filter.urls[url] = struct{}{}
	}
	return nil
}

// parseFilterConfig parses a filter configuration. The following forms are accepted:
// - Satellite ID followed by @
// - Satellite host
// - Full Satellite URL (i.e. id@host:port)
func parseFilterConfig(s string) (SatelliteURL, error) {
	url, err := storj.ParseNodeURL(s)
	if err != nil {
		return SatelliteURL{}, Error.New("invalid filter: %v", err)
	}

	switch {
	case url.ID.IsZero() && url.Address != "":
		// Just the address was specified. Ensure it does not have a port.
		_, _, err := net.SplitHostPort(url.Address)
		if err == nil {
			return SatelliteURL{}, Error.New("host filter must not specify a port")
		}
		return SatelliteURL{
			Host: url.Address,
		}, nil
	case !url.ID.IsZero() && url.Address == "":
		// Just the ID was specified.
		return SatelliteURL{
			ID: url.ID,
		}, nil
	}

	// storj.ParseNodeURL will have already verified that the address is
	// well-formed, so if SplitHostPort fails it should be due to the address
	// not having a port.
	host, portStr, err := net.SplitHostPort(url.Address)
	if err != nil {
		return SatelliteURL{}, Error.New("satellite URL filter must specify a port")
	}

	// Port should already be numeric so this shouldn't fail, but just in case.
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return SatelliteURL{}, Error.New("satellite URL filter port is not numeric")
	}

	return SatelliteURL{
		ID:   url.ID,
		Host: host,
		Port: port,
	}, nil
}
