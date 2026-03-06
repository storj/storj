// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// IdentityConfig extends identity.Config with direct PEM content fields.
// When Cert and Key are set, they are used directly instead of reading from CertPath/KeyPath.
type IdentityConfig struct {
	identity.Config

	Cert string `help:"PEM-encoded certificate chain (alternative to cert-path)" default:""`
	Key  string `help:"PEM-encoded private key (alternative to key-path)" default:""`
}

// Load loads a FullIdentity from the config.
// If Cert and Key are set directly, they are used as PEM content.
// Otherwise, falls back to loading from CertPath/KeyPath.
func (ic IdentityConfig) Load() (*identity.FullIdentity, error) {
	if ic.Cert != "" && ic.Key != "" {
		return identity.FullIdentityFromPEM([]byte(ic.Cert), []byte(ic.Key))
	}
	return ic.Config.Load()
}

// IdentityModule provides identity related components for modular setup.
func IdentityModule(ball *mud.Ball) {
	config.RegisterConfig[IdentityConfig](ball, "identity")
	mud.Provide[*identity.FullIdentity](ball, func(cfg *IdentityConfig) (*identity.FullIdentity, error) {
		return cfg.Load()
	})
	mud.View[*identity.FullIdentity, storj.NodeID](ball, func(fid *identity.FullIdentity) storj.NodeID {
		return fid.ID
	})
}
