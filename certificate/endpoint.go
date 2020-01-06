// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificate

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/certificate/authorization"
)

// Endpoint implements pb.CertificatesServer.
type Endpoint struct {
	sanitizer       *errs2.LoggingSanitizer
	log             *zap.Logger
	ca              *identity.FullCertificateAuthority
	authorizationDB *authorization.DB
	minDifficulty   uint16
}

// NewEndpoint creates a new certificate signing gRPC server.
func NewEndpoint(log *zap.Logger, ca *identity.FullCertificateAuthority, authorizationDB *authorization.DB, minDifficulty uint16) *Endpoint {
	codeMap := errs2.CodeMap{
		&authorization.ErrNotFound:       rpcstatus.Unauthenticated,
		&authorization.ErrInvalidClaim:   rpcstatus.InvalidArgument,
		&authorization.ErrInvalidToken:   rpcstatus.InvalidArgument,
		&authorization.ErrAlreadyClaimed: rpcstatus.AlreadyExists,
	}
	sanitizer := errs2.NewLoggingSanitizer(&Error, log, codeMap)

	return &Endpoint{
		sanitizer:       sanitizer,
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
		return nil, endpoint.sanitizer.Error(msg, err)
	}

	peerIdent, err := identity.PeerIdentityFromPeer(peer)
	if err != nil {
		msg := "error getting peer identity"
		return nil, endpoint.sanitizer.Error(msg, err)
	}

	signedPeerCA, err := endpoint.ca.Sign(peerIdent.CA)
	if err != nil {
		msg := "error signing peer CA"
		return nil, endpoint.sanitizer.Error(msg, err)
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
		return nil, endpoint.sanitizer.Error(msg, err)
	}

	difficulty, err := peerIdent.ID.Difficulty()
	if err != nil {
		msg := "error checking difficulty"
		return nil, endpoint.sanitizer.Error(msg, err)
	}
	token, err := authorization.ParseToken(req.AuthToken)
	if err != nil {
		msg := "error parsing auth token"
		return nil, endpoint.sanitizer.Error(msg, err)
	}
	tokenFormatter := authorization.Authorization{
		Token: *token,
	}
	endpoint.log.Info("certificate successfully signed",
		zap.Stringer("Node ID", peerIdent.ID),
		zap.Uint16("difficulty", difficulty),
		zap.Stringer("truncated token", tokenFormatter),
	)

	return &pb.SigningResponse{
		Chain: signedChainBytes,
	}, nil
}
