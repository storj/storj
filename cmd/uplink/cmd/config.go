// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/uplink"
)

var mon = monkit.Package()

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	UserAgent   string        `help:"User-Agent used for connecting to the satellite" default:""`
	DialTimeout time.Duration `help:"timeout for dials" default:"0h2m00s"`
}

// Config uplink configuration.
type Config struct {
	AccessConfig
	Client ClientConfig
}

// AccessConfig holds information about which accesses exist and are selected.
type AccessConfig struct {
	Accesses map[string]string `internal:"true"`
	Access   string            `help:"the serialized access, or name of the access to use" default:"" basic-help:"true"`

	// used for backward compatibility
	Scopes map[string]string `internal:"true"` // deprecated
	Scope  string            `internal:"true"` // deprecated
}

// normalize looks for usage of deprecated config values and sets the respective
// non-deprecated config values accordingly and returns them in a copy of the config.
func (a AccessConfig) normalize() (_ AccessConfig) {
	// fallback to scope if access not found
	if a.Access == "" {
		a.Access = a.Scope
	}

	if a.Accesses == nil {
		a.Accesses = make(map[string]string)
	}

	// fallback to scopes if accesses not found
	if len(a.Accesses) == 0 {
		for name, access := range a.Scopes {
			a.Accesses[name] = access
		}
	}

	return a
}

// GetAccess returns the appropriate access for the config.
func (a AccessConfig) GetAccess() (_ *uplink.Access, err error) {
	defer mon.Task()(nil)(&err)

	a = a.normalize()

	access, err := a.GetNamedAccess(a.Access)
	if err != nil {
		return nil, err
	}
	if access != nil {
		return access, nil
	}

	// Otherwise, try to load the access name as a serialized access.
	return uplink.ParseAccess(a.Access)
}

// GetNamedAccess returns named access if exists.
func (a AccessConfig) GetNamedAccess(name string) (_ *uplink.Access, err error) {
	// if an access exists for that name, try to load it.
	if data, ok := a.Accesses[name]; ok {
		return uplink.ParseAccess(data)
	}
	return nil, nil
}

// IsSerializedAccess returns whether the passed access is a serialized
// access string or not.
func IsSerializedAccess(access string) bool {
	_, err := uplink.ParseAccess(access)
	return err == nil
}
