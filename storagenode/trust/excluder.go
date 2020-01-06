// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"net"
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

var (
	// ErrExclusion is an error class for exclusion related errors
	ErrExclusion = errs.Class("exclusion")
)

// NewExcluder takes a configuration string and returns an excluding Rule.
// Accepted forms are 1) a Satellite ID followed by '@', 2) a hostname or IP
// address, 3) a full Satellite URL.
func NewExcluder(config string) (Rule, error) {
	url, err := parseExcluderConfig(config)
	if err != nil {
		return nil, err
	}

	switch {
	case url.Host == "":
		return NewIDExcluder(url.ID), nil
	case url.ID.IsZero():
		return NewHostExcluder(url.Host), nil
	default:
		return NewURLExcluder(url), nil
	}
}

// URLExcluder excludes matching URLs
type URLExcluder struct {
	url SatelliteURL
}

// NewURLExcluder returns a new URLExcluder
func NewURLExcluder(url SatelliteURL) *URLExcluder {
	url.Host = normalizeHost(url.Host)
	return &URLExcluder{
		url: url,
	}
}

// IsTrusted returns true if the given Satellite is trusted and false otherwise
func (excluder *URLExcluder) IsTrusted(url SatelliteURL) bool {
	url.Host = normalizeHost(url.Host)
	return excluder.url != url
}

// String returns a string representation of the excluder
func (excluder *URLExcluder) String() string {
	return excluder.url.String()
}

// IDExcluder excludes URLs matching a given URL
type IDExcluder struct {
	id storj.NodeID
}

// NewIDExcluder returns a new IDExcluder
func NewIDExcluder(id storj.NodeID) *IDExcluder {
	return &IDExcluder{
		id: id,
	}
}

// IsTrusted returns true if the given Satellite is trusted and false otherwise
func (excluder *IDExcluder) IsTrusted(url SatelliteURL) bool {
	return excluder.id != url.ID
}

// String returns a string representation of the excluder
func (excluder *IDExcluder) String() string {
	return excluder.id.String() + "@"
}

// HostExcluder excludes URLs that match a given host. If the host is a domain
// name then URLs in a subdomain of that domain are excluded as well.
type HostExcluder struct {
	host   string
	suffix string
}

// NewHostExcluder returns a new HostExcluder
func NewHostExcluder(host string) *HostExcluder {
	host = normalizeHost(host)

	// If it appears to be a domain name (i.e. has a dot) then configure the
	// suffix as well
	var suffix string
	if strings.ContainsRune(host, '.') {
		suffix = "." + host
	}
	return &HostExcluder{
		host:   host,
		suffix: suffix,
	}
}

// IsTrusted returns true if the given Satellite is trusted and false otherwise
func (excluder *HostExcluder) IsTrusted(url SatelliteURL) bool {
	host := normalizeHost(url.Host)
	if excluder.host == host {
		return false
	}

	if excluder.suffix != "" && strings.HasSuffix(host, excluder.suffix) {
		return false
	}
	return true
}

// String returns a string representation of the excluder
func (excluder *HostExcluder) String() string {
	return excluder.host
}

// parseExcluderConfig parses a excluder configuration. The following forms are accepted:
// - Satellite ID followed by @
// - Satellite host
// - Full Satellite URL (i.e. id@host:port)
func parseExcluderConfig(s string) (SatelliteURL, error) {
	url, err := storj.ParseNodeURL(s)
	if err != nil {
		return SatelliteURL{}, ErrExclusion.Wrap(err)
	}

	switch {
	case url.ID.IsZero() && url.Address != "":
		// Just the address was specified. Ensure it does not have a port.
		_, _, err := net.SplitHostPort(url.Address)
		if err == nil {
			return SatelliteURL{}, ErrExclusion.New("host exclusion must not include a port")
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
		return SatelliteURL{}, ErrExclusion.New("satellite URL exclusion must specify a port")
	}

	// Port should already be numeric so this shouldn't fail, but just in case.
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return SatelliteURL{}, ErrExclusion.New("satellite URL exclusion port is not numeric")
	}

	return SatelliteURL{
		ID:   url.ID,
		Host: host,
		Port: port,
	}, nil
}
