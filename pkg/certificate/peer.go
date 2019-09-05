// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificate

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/pkg/certificate/authorization"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
)

var (
	mon = monkit.Package()

	// Error is the default error class for the certificates peer.
	Error = errs.Class("certificates peer error")
)

// Config is the global certificates config.
type Config struct {
	Identity identity.Config
	Server   server.Config

	Signer         identity.FullCAConfig
	Authorizations authorization.Config

	MinDifficulty uint `default:"30" help:"minimum difficulty of the requester's identity required to claim an authorization"`
}

// Peer is the certificates server.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity

	Server *server.Server

	// services and endpoints
	Certificates struct {
		AuthorizationDB *authorization.DB
		Endpoint        *Endpoint
	}
}

// New creates a new certificates peer.
func New(log *zap.Logger, ident *identity.FullIdentity, ca *identity.FullCertificateAuthority, authorizationDB *authorization.DB, revocationDB *revocation.DB, config *Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: ident,
	}

	{
		log.Debug("Starting listener and server")
		sc := config.Server

		options, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, Error.Wrap(errs.Combine(err, peer.Close()))
		}

		peer.Server, err = server.New(log.Named("server"), options, sc.Address, sc.PrivateAddress, nil)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	peer.Certificates.AuthorizationDB = authorizationDB
	peer.Certificates.Endpoint = NewEndpoint(log.Named("certificates"), ca, authorizationDB, uint16(config.MinDifficulty))
	pb.RegisterCertificatesServer(peer.Server.GRPC(), peer.Certificates.Endpoint)

	return peer, nil
}

// Run runs the certificates peer until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return errs2.IgnoreCanceled(peer.Server.Run(ctx))
}

// Close closes all resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}

	if peer.Certificates.AuthorizationDB != nil {
		errlist.Add(peer.Certificates.AuthorizationDB.Close())
	}

	return Error.Wrap(errlist.Err())
}
