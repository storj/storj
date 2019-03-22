// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"storj.io/storj/pkg/identity"
	ul "storj.io/storj/uplink"
)

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	id            *identity.FullIdentity
	satelliteAddr string
	config        ul.Config
}

// Access returns a pointer to an Access for bucket operations to occur on
func (u *Uplink) Access(ctx context.Context, permissions Permissions) *Access {
	// TODO (dylan): Parse permissions here
	return &Access{
		Uplink: u,
	}
}

// NewUplink returns a pointer to a new Uplink or an error
func NewUplink(identity *identity.FullIdentity, satelliteAddr string, cfg ul.Config) *Uplink {
	return &Uplink{
		id:            identity,
		satelliteAddr: satelliteAddr,
		config:        cfg,
	}
}
