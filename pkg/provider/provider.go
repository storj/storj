// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"net"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/storage"
)

//ErrSetup is setup error
var (
	ErrSetup = errs.Class("setup error")
)

// Responsibility represents a specific gRPC method collection to be registered
// on a shared gRPC server. PointerDB, OverlayCache, PieceStore, Kademlia,
// StatDB, etc. are all examples of Responsibilities.
type Responsibility interface {
	Run(ctx context.Context, server *Provider) error
}

// Provider represents a bundle of responsibilities defined by a specific ID.
// Examples of providers are the heavy client, the farmer, and the gateway.
type Provider struct {
	lis      net.Listener
	g        *grpc.Server
	next     []Responsibility
	identity *FullIdentity
}

// NewProvider creates a Provider out of an Identity, a net.Listener, and a set
// of responsibilities.
func NewProvider(identity *FullIdentity, lis net.Listener,
	responsibilities ...Responsibility) (*Provider, error) {
	// NB: talk to anyone with an identity
	ident, err := identity.ServerOption()
	if err != nil {
		return nil, err
	}

	// TODO: use ident
	_ = ident

	return &Provider{
		lis: lis,
		g: grpc.NewServer(
			grpc.StreamInterceptor(streamInterceptor),
			grpc.UnaryInterceptor(unaryInterceptor),
			//			ident,  TODO
		),
		next:     responsibilities,
		identity: identity,
	}, nil
}

// SetupIdentity ensures a CA and identity exist and returns a config overrides map
func SetupIdentity(ctx context.Context, c CASetupConfig, i IdentitySetupConfig) error {
	if s := c.Stat(); s == NoCertNoKey || c.Overwrite {
		t, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return errs.Wrap(err)
		}
		ctx, _ = context.WithTimeout(ctx, t)

		// Load or create a certificate authority
		ca, err := c.Create(ctx, 4)
		if err != nil {
			return err
		}

		if s := i.Stat(); s == NoCertNoKey || i.Overwrite {
			// Create identity from new CA
			_, err = i.Create(ca)
			if err != nil {
				return err
			}

			return nil
		} else {
			return ErrSetup.New("identity file(s) exist: %s", s)
		}
	} else {
		return ErrSetup.New("certificate authority file(s) exist: %s", s)
	}
}

// Identity returns the provider's identity
func (p *Provider) Identity() *FullIdentity { return p.identity }

// GRPC returns the provider's gRPC server for registration purposes
func (p *Provider) GRPC() *grpc.Server { return p.g }

// Close shuts down the provider
func (p *Provider) Close() error {
	p.g.GracefulStop()
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

	return p.g.Serve(p.lis)
}

func streamInterceptor(srv interface{}, ss grpc.ServerStream,
	info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	err = handler(srv, ss)
	if err != nil {
		// modified for wrong file downloads
		// to not zap errors
		if storage.ErrKeyNotFound.Has(err) {
			return err
		}
		zap.S().Errorf("%+v", err)
	}
	return err
}

func unaryInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{},
	err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		// modified for wrong file downloads
		// to not zap errors
		if status.Code(err) == codes.NotFound {
			return resp, err
		}
		zap.S().Errorf("%+v", err)
	}
	return resp, err
}
