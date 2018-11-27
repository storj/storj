// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/storage"
)

var (
	// ErrSetup is returned when there's an error with setup
	ErrSetup = errs.Class("setup error")
)

// Responsibility represents a specific gRPC method collection to be registered
// on a shared gRPC server. PointerDB, OverlayCache, PieceStore, Kademlia,
// StatDB, etc. are all examples of Responsibilities.
type Responsibility interface {
	Run(ctx context.Context, server *Provider) error
}

// Provider represents a bundle of responsibilities defined by a specific ID.
// Examples of providers are the heavy client, the storagenode, and the gateway.
type Provider struct {
	lis      net.Listener
	grpc     *grpc.Server
	next     []Responsibility
	identity *FullIdentity
}

// NewProvider creates a Provider out of an Identity, a net.Listener, a UnaryInterceptorProvider and
// a set of responsibilities.
func NewProvider(identity *FullIdentity, lis net.Listener, interceptor grpc.UnaryServerInterceptor,
	responsibilities ...Responsibility) (*Provider, error) {
	// NB: talk to anyone with an identity
	ident, err := identity.ServerOption(peertls.VerifyCAWhitelist(
		identity.PeerCAWhitelist,
		identity.VerifyAuthExtSig,
	))
	if err != nil {
		return nil, err
	}

	unaryInterceptor := unaryInterceptor
	if interceptor != nil {
		unaryInterceptor = combineInterceptors(unaryInterceptor, interceptor)
	}

	return &Provider{
		lis: lis,
		grpc: grpc.NewServer(
			grpc.StreamInterceptor(streamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			ident,
		),
		next:     responsibilities,
		identity: identity,
	}, nil
}

// SetupIdentity ensures a CA and identity exist and returns a config overrides map
func SetupIdentity(ctx context.Context, c CASetupConfig, i IdentitySetupConfig) error {
	if s := c.Status(); s != NoCertNoKey && !c.Overwrite {
		return ErrSetup.New("certificate authority file(s) exist: %s", s)
	}

	t, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return errs.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	// Load or create a certificate authority
	ca, err := c.Create(ctx)
	if err != nil {
		return err
	}

	if s := c.Status(); s != NoCertNoKey && !c.Overwrite {
		return ErrSetup.New("identity file(s) exist: %s", s)
	}

	// Create identity from new CA
	_, err = i.Create(ca)
	return err
}

// Identity returns the provider's identity
func (p *Provider) Identity() *FullIdentity { return p.identity }

// GRPC returns the provider's gRPC server for registration purposes
func (p *Provider) GRPC() *grpc.Server { return p.grpc }

// Close shuts down the provider
func (p *Provider) Close() error {
	p.grpc.GracefulStop()
	return nil
}

// Run will run the provider and all of its responsibilities
func (p *Provider) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	// are there any unstarted responsibilities? start those first. the
	// responsibilities know to call Run again once they're ready.
	if len(p.next) > 0 {
		next := p.next[0]
		p.next = p.next[1:]
		return next.Run(ctx, p)
	}

	return p.grpc.Serve(p.lis)
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	err = handler(srv, ss)
	if err != nil {
		// no zap errors for canceled or wrong file downloads
		if storage.ErrKeyNotFound.Has(err) ||
			status.Code(err) == codes.Canceled ||
			status.Code(err) == codes.Unavailable ||
			err == io.EOF {
			return err
		}
		zap.S().Errorf("%+v", err)
	}
	return err
}

func unaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
	err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		// no zap errors for wrong file downloads
		if status.Code(err) == codes.NotFound {
			return resp, err
		}
		zap.S().Errorf("%+v", err)
	}
	return resp, err
}

func combineInterceptors(a, b grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return a(ctx, req, info, func(actx context.Context, areq interface{}) (interface{}, error) {
			return b(actx, areq, info, func(bctx context.Context, breq interface{}) (interface{}, error) {
				return handler(bctx, breq)
			})
		})
	}
}
