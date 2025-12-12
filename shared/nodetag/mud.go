// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetag

import (
	"storj.io/common/identity"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[Authority](ball, func(identity *identity.FullIdentity, cfg *Config) (Authority, error) {
		return LoadAuthorities(identity.PeerIdentity(), cfg.TagAuthorities)
	})
	config.RegisterConfig[Config](ball, "")
}
