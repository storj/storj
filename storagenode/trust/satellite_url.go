// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"net"
	"strconv"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
)

var (
	// ErrSatelliteURL is an error class for satellite URL related errors
	ErrSatelliteURL = errs.Class("invalid satellite URL")
)

// SatelliteURL represents a Satellite URL
type SatelliteURL struct {
	ID   storj.NodeID `json:"id"`
	Host string       `json:"host"`
	Port int          `json:"port"`
}

// Address returns the address (i.e. host:port) of the Satellite
func (u *SatelliteURL) Address() string {
	return net.JoinHostPort(u.Host, strconv.Itoa(u.Port))
}

// NodeURL returns a full Node URL to the Satellite
func (u *SatelliteURL) NodeURL() storj.NodeURL {
	return storj.NodeURL{
		ID:      u.ID,
		Address: u.Address(),
	}
}

// String returns a string representation of the Satellite URL
func (u *SatelliteURL) String() string {
	return u.ID.String() + "@" + u.Address()
}

// ParseSatelliteURL parses a Satellite URL. For the purposes of the trust list,
// the Satellite URL MUST contain both an ID and port designation.
func ParseSatelliteURL(s string) (SatelliteURL, error) {
	url, err := storj.ParseNodeURL(s)
	if err != nil {
		return SatelliteURL{}, ErrSatelliteURL.Wrap(err)
	}
	if url.ID.IsZero() {
		return SatelliteURL{}, ErrSatelliteURL.New("must contain an ID")
	}

	if url.Address == "" {
		return SatelliteURL{}, ErrSatelliteURL.New("must specify the host:port")
	}

	// storj.ParseNodeURL will have already verified that the address is
	// well-formed, so if SplitHostPort fails it should be due to the address
	// not having a port
	host, portStr, err := net.SplitHostPort(url.Address)
	if err != nil {
		return SatelliteURL{}, ErrSatelliteURL.New("must specify the port")
	}

	// Port should already be numeric so this shouldn't fail, but just in case.
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return SatelliteURL{}, ErrSatelliteURL.New("port is not numeric")
	}

	return SatelliteURL{
		ID:   url.ID,
		Host: host,
		Port: port,
	}, nil
}
