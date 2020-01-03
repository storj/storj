// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificate

import (
	"context"
	"net"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/storj/certificate/authorization"
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

	Signer            identity.FullCAConfig
	AuthorizationDB   authorization.DBConfig
	AuthorizationAddr string `default:"127.0.0.1:9000" help:"address for authorization http proxy to listen on"`

	MinDifficulty uint `default:"36" help:"minimum difficulty of the requester's identity required to claim an authorization"`
}

// Peer is the certificates server.
type Peer struct {
	// core dependencies
	Log      *zap.Logger
	Identity *identity.FullIdentity

	Server          *server.Server
	AuthorizationDB *authorization.DB

	// services and endpoints
	Certificate struct {
		Endpoint *Endpoint
	}

	Authorization struct {
		Listener net.Listener
		Service  *authorization.Service
		Endpoint *authorization.Endpoint
	}
}

// New creates a new certificates peer.
func New(log *zap.Logger, ident *identity.FullIdentity, ca *identity.FullCertificateAuthority, authorizationDB *authorization.DB, revocationDB *revocation.DB, config *Config) (*Peer, error) {
	peer := &Peer{
		Log:      log,
		Identity: ident,
	}

	{ // setup server
		log.Debug("Starting listener and server")
		sc := config.Server

		tlsOptions, err := tlsopts.NewOptions(peer.Identity, sc.Config, revocationDB)
		if err != nil {
			return nil, Error.Wrap(errs.Combine(err, peer.Close()))
		}

		peer.Server, err = server.New(log.Named("server"), tlsOptions, sc.Address, sc.PrivateAddress, nil)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	peer.AuthorizationDB = authorizationDB

	peer.Certificate.Endpoint = NewEndpoint(log.Named("certificate"), ca, authorizationDB, uint16(config.MinDifficulty))
	pb.RegisterCertificatesServer(peer.Server.GRPC(), peer.Certificate.Endpoint)
	pb.DRPCRegisterCertificates(peer.Server.DRPC(), peer.Certificate.Endpoint)

	var err error
	peer.Authorization.Listener, err = net.Listen("tcp", config.AuthorizationAddr)
	if err != nil {
		return nil, errs.Combine(err, peer.Close())
	}

	authorizationService := authorization.NewService(log, authorizationDB)
	peer.Authorization.Endpoint = authorization.NewEndpoint(log.Named("authorization"), authorizationService, peer.Authorization.Listener)

	return peer, nil
}

// Run runs the certificates peer until it's either closed or it errors.
func (peer *Peer) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	group := errgroup.Group{}

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Server.Run(ctx))
	})

	group.Go(func() error {
		return errs2.IgnoreCanceled(peer.Authorization.Endpoint.Run(ctx))
	})

	return group.Wait()
}

// Close closes all resources.
func (peer *Peer) Close() error {
	var errlist errs.Group

	if peer.Authorization.Endpoint != nil {
		errlist.Add(peer.Authorization.Endpoint.Close())
	}

	if peer.AuthorizationDB != nil {
		errlist.Add(peer.AuthorizationDB.Close())
	}

	if peer.Server != nil {
		errlist.Add(peer.Server.Close())
	}

	return Error.Wrap(errlist.Err())
}
