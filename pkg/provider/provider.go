// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"context"
	"net"
	"path/filepath"
	"time"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
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
	s, err := identity.ServerOption(0)
	if err != nil {
		return nil, err
	}

	return &Provider{
		lis:      lis,
		g:        grpc.NewServer(s),
		next:     responsibilities,
		identity: identity,
	}, nil
}

func SetupIdentityPaths(basePath string, c *CAConfig, i *IdentityConfig) {
	c.CertPath = filepath.Join(basePath, "ca.cert")
	c.KeyPath = filepath.Join(basePath, "ca.key")
	i.CertPath = filepath.Join(basePath, "identity.cert")
	i.KeyPath = filepath.Join(basePath, "identity.key")
}

// SetupIdentity ensures a CA and identity exist and returns a config overrides map
func SetupIdentity(ctx context.Context, c CASetupConfig, i IdentitySetupConfig) (map[string]interface{}, error) {
	t, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	ctx, _ = context.WithTimeout(ctx, t)

	// Load or create a certificate authority
	ca, n, err := c.LoadOrCreate(ctx, 4)
	if err != nil {
		return nil, err
	}
	if n {
		// Create identity from new CA
		_, err = i.Create(ca)
		if err != nil {
			return nil, err
		}
	} else {
		// Load or create identity from existing CA
		_, err = i.LoadOrCreate(ca)
		if err != nil {
			return nil, err
		}
	}

	o := map[string]interface{}{
		"ca.cert-path":       c.CertPath,
		"ca.key-path":        "",
		"ca.difficulty":      c.Difficulty,
		"ca.version":         c.Version,
		"identity.cert-path": i.CertPath,
		"identity.key-path":  i.KeyPath,
		"identity.version":   i.Version,
		"identity.address":   i.Address,
	}
	return o, nil
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
