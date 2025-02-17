// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// IdentityModule provides identity related components for modular setup.
func IdentityModule(ball *mud.Ball) {
	config.RegisterConfig[identity.Config](ball, "identity")
	mud.Provide[*identity.FullIdentity](ball, func(cfg *identity.Config) (*identity.FullIdentity, error) {
		return cfg.Load()
	})
	mud.View[*identity.FullIdentity, storj.NodeID](ball, func(fid *identity.FullIdentity) storj.NodeID {
		return fid.ID
	})
}
