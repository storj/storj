// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"os"
	"path/filepath"

	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/testrand"
	"storj.io/storj/pkg/server"
)

// DefaultReferralManagerServer implements the default behavior of a mock referral manager
type DefaultReferralManagerServer struct {
	tokenCount int
}

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

// GetTokens implements a mock GetTokens endpoint that returns a number of referral tokens. By default, it returns 0 tokens.
func (server *DefaultReferralManagerServer) GetTokens(ctx context.Context, req *pb.GetTokensRequest) (*pb.GetTokensResponse, error) {
	tokens := make([][]byte, server.tokenCount)
	for i := 0; i < server.tokenCount; i++ {
		uuid := testrand.UUID()
		tokens[i] = uuid[:]
	}
	return &pb.GetTokensResponse{
		TokenSecrets: tokens,
	}, nil
}

// RedeemToken implements a mock RedeemToken endpoint.
func (server *DefaultReferralManagerServer) RedeemToken(ctx context.Context, req *pb.RedeemTokenRequest) (*pb.RedeemTokenResponse, error) {
	return &pb.RedeemTokenResponse{}, nil
}

// SetTokenCount sets the number of tokens GetTokens endpoint should return.
func (server *DefaultReferralManagerServer) SetTokenCount(tokenCount int) {
	server.tokenCount = tokenCount
}
