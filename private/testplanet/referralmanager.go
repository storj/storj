// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"os"
	"path/filepath"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
)

// newReferralManager initializes a referral manager server
func (planet *Planet) newReferralManager() (*server.Server, error) {
	prefix := "referralmanager"
	log := planet.log.Named(prefix)
	referralmanagerDir := filepath.Join(planet.directory, prefix)

	if err := os.MkdirAll(referralmanagerDir, 0700); err != nil {
		return nil, err
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	config := server.Config{
		Address:        "127.0.0.1:0",
		PrivateAddress: "127.0.0.1:0",

		Config: tlsopts.Config{
			RevocationDBURL:    "bolt://" + filepath.Join(referralmanagerDir, "revocation.db"),
			UsePeerCAWhitelist: true,
			PeerIDVersions:     "*",
			Extensions: extensions.Config{
				Revocation:          false,
				WhitelistSignedLeaf: false,
			},
		},
	}

	var endpoints pb.ReferralManagerServer
	// only create a referral manager server if testplanet was reconfigured with a custom referral manager endpoint
	if planet.config.Reconfigure.ReferralManagerServer != nil {
		endpoints = planet.config.Reconfigure.ReferralManagerServer(log)
	} else {
		return nil, nil
	}
	tlsOptions, err := tlsopts.NewOptions(identity, config.Config, nil)
	if err != nil {
		return nil, err
	}

	referralmanager, err := server.New(log, tlsOptions, config.Address, config.PrivateAddress, nil)
	if err != nil {
		return nil, err
	}
	pb.DRPCRegisterReferralManager(referralmanager.DRPC(), endpoints)

	log.Debug("id=" + identity.ID.String() + " addr=" + referralmanager.Addr().String())
	return referralmanager, nil
}
