// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc/peer"

	"storj.io/storj/pkg/certificates/authorizations"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
)

// Certificates implements pb.CertificatesServer
type Certificates struct {
	log             *zap.Logger
	ca              *identity.FullCertificateAuthority
	authorizationDB *authorizations.DB
	minDifficulty   uint16
}

// NewCertificatesServer creates a new certificate signing grpc server
func NewCertificatesServer(log *zap.Logger, ident *identity.FullIdentity, ca *identity.FullCertificateAuthority, authorizationDB *authorizations.DB, minDifficulty uint16) *Certificates {
	return &Certificates{
		log:             log,
		ca:              ca,
		authorizationDB: authorizationDB,
		minDifficulty:   minDifficulty,
	}
}

// Sign signs the CA certificate of the remote peer's identity with the `certs.ca` certificate.
// Returns a certificate chain consisting of the remote peer's CA followed by the `certs.ca` chain.
func (certs Certificates) Sign(ctx context.Context, req *pb.SigningRequest) (_ *pb.SigningResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	grpcPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, authorizations.Error.New("unable to get peer from context")
	}

	peerIdent, err := identity.PeerIdentityFromPeer(grpcPeer)
	if err != nil {
		return nil, err
	}

	signedPeerCA, err := certs.ca.Sign(peerIdent.CA)
	if err != nil {
		return nil, err
	}

	signedChainBytes := [][]byte{signedPeerCA.Raw, certs.ca.Cert.Raw}
	signedChainBytes = append(signedChainBytes, certs.ca.RawRestChain()...)
	err = certs.authorizationDB.Claim(ctx, &authorizations.ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    signedChainBytes,
		MinDifficulty: certs.minDifficulty,
	})
	if err != nil {
		return nil, err
	}

	difficulty, err := peerIdent.ID.Difficulty()
	if err != nil {
		certs.log.Error("error checking difficulty", zap.Error(err))
	}
	token, err := authorizations.ParseToken(req.AuthToken)
	if err != nil {
		certs.log.Error("error parsing auth token", zap.Error(err))
	}
	tokenFormatter := authorizations.Authorization{
		Token: *token,
	}
	certs.log.Info("certificate successfully signed",
		zap.Stringer("node ID", peerIdent.ID),
		zap.Uint16("difficulty", difficulty),
		zap.Stringer("truncated token", tokenFormatter),
	)

	return &pb.SigningResponse{
		Chain: signedChainBytes,
	}, nil
}
