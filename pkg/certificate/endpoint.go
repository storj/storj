// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificate

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/certificate/authorization"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc/rpcpeer"
	"storj.io/storj/pkg/rpc/rpcstatus"
)

// Endpoint implements pb.CertificatesServer.
type Endpoint struct {
	log             *zap.Logger
	ca              *identity.FullCertificateAuthority
	authorizationDB *authorization.DB
	minDifficulty   uint16
}

// NewEndpoint creates a new certificate signing gRPC server.
func NewEndpoint(log *zap.Logger, ca *identity.FullCertificateAuthority, authorizationDB *authorization.DB, minDifficulty uint16) *Endpoint {
	return &Endpoint{
		log:             log,
		ca:              ca,
		authorizationDB: authorizationDB,
		minDifficulty:   minDifficulty,
	}
}

// Sign signs the CA certificate of the remote peer's identity with the `certs.ca` certificate.
// Returns a certificate chain consisting of the remote peer's CA followed by the CA chain.
func (endpoint Endpoint) Sign(ctx context.Context, req *pb.SigningRequest) (_ *pb.SigningResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	peer, err := rpcpeer.FromContext(ctx)
	if err != nil {
		msg := "error getting peer from context"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}

	peerIdent, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		msg := "error getting peer identity"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}

	signedPeerCA, err := endpoint.ca.Sign(peerIdent.CA)
	if err != nil {
		msg := "error signing peer CA"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}

	signedChainBytes := [][]byte{signedPeerCA.Raw, endpoint.ca.Cert.Raw}
	signedChainBytes = append(signedChainBytes, endpoint.ca.RawRestChain()...)
	err = endpoint.authorizationDB.Claim(ctx, &authorization.ClaimOpts{
		Req:           req,
		Peer:          peer,
		ChainBytes:    signedChainBytes,
		MinDifficulty: endpoint.minDifficulty,
	})
	if err != nil {
		msg := "error claiming authorization"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}

	difficulty, err := peerIdent.ID.Difficulty()
	if err != nil {
		msg := "error checking difficulty"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}
	token, err := authorization.ParseToken(req.AuthToken)
	if err != nil {
		msg := "error parsing auth token"
		endpoint.log.Error(msg, zap.Error(err))
		return nil, internalErr(msg)
	}
	tokenFormatter := authorization.Authorization{
		Token: *token,
	}
	endpoint.log.Info("certificate successfully signed",
		zap.Stringer("node ID", peerIdent.ID),
		zap.Uint16("difficulty", difficulty),
		zap.Stringer("truncated token", tokenFormatter),
	)

	return &pb.SigningResponse{
		Chain: signedChainBytes,
	}, nil
}

func internalErr(msg string) error {
	return rpcstatus.Error(rpcstatus.Internal, Error.New(msg).Error())
}
