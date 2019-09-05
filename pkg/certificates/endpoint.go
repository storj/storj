// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"go.uber.org/zap"
	"google.golang.org/grpc/peer"

	"storj.io/storj/pkg/certificates/authorization"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// Endpoint implements pb.CertificatesServer.
type Endpoint struct {
	log             *zap.Logger
	ca              *identity.FullCertificateAuthority
	authorizationDB *authorization.DB
	minDifficulty   uint16
}

// NewEndpoint creates a new certificate signing gRPC server.
func NewEndpoint(log *zap.Logger, ident *identity.FullIdentity, ca *identity.FullCertificateAuthority, authorizationDB *authorization.DB, minDifficulty uint16) *Endpoint {
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
	grpcPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, internalErr(Error.New("unable to get peer from context"))
	}

	peerIdent, err := identity.PeerIdentityFromPeer(grpcPeer)
	if err != nil {
		return nil, internalErr(err)
	}

	signedPeerCA, err := endpoint.ca.Sign(peerIdent.CA)
	if err != nil {
		return nil, internalErr(err)
	}

	signedChainBytes := [][]byte{signedPeerCA.Raw, endpoint.ca.Cert.Raw}
	signedChainBytes = append(signedChainBytes, endpoint.ca.RawRestChain()...)
	err = endpoint.authorizationDB.Claim(ctx, &authorization.ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    signedChainBytes,
		MinDifficulty: endpoint.minDifficulty,
	})
	if err != nil {
		return nil, internalErr(err)
	}

	difficulty, err := peerIdent.ID.Difficulty()
	if err != nil {
		endpoint.log.Error("error checking difficulty", zap.Error(err))
	}
	token, err := authorization.ParseToken(req.AuthToken)
	if err != nil {
		endpoint.log.Error("error parsing auth token", zap.Error(err))
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

func internalErr(err error) error {
	return status.Error(codes.Internal, Error.Wrap(err).Error())
}
