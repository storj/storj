// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testidentity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcmd"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"

	"storj.io/storj/pkg/provider"
)

// NewTestIdentity is a helper function to generate new node identities with
// correct difficulty and concurrency
func NewTestIdentity(ctx context.Context) (*provider.FullIdentity, error) {
	ca, err := provider.NewCA(ctx, provider.NewCAOptions{
		Difficulty:  4,
		Concurrency: 1,
	})
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}

// NewTestCA returns a ca with a default difficulty and concurrency for use in tests
func NewTestCA(ctx context.Context) (*provider.FullCertificateAuthority, error) {
	return provider.NewCA(ctx, provider.NewCAOptions{
		Difficulty:  4,
		Concurrency: 1,
	})
}

func NewTestIdentityFromCmd(t *testing.T, cmdIdentity *testcmd.Cmd, caConfig identity.FullCAConfig, identConfig identity.Config) {
	assert := assert.New(t)

	// Create CA
	// -- ensure CA doesn't already exist
	_, err := caConfig.Load()
	if !assert.True(peertls.ErrNotExist.Has(err)) {
		t.Fatal("expected CA to not exist")
	}

	// -- create CA
	err = cmdIdentity.Run(
		"ca", "new",
		"--ca.cert-path", caConfig.CertPath,
		"--ca.key-path", caConfig.KeyPath,
	)
	if !assert.NoError(err) {
		t.Fatal(err)
	}

	// Create Identity
	// -- ensure identity doesn't already exist
	_, err = identConfig.Load()
	if !assert.True(peertls.ErrNotExist.Has(err)) {
		t.Fatal("expected identity to not exist")
	}

	// -- create identity
	err = cmdIdentity.Run(
		"id", "new",
		"--ca.cert-path", caConfig.CertPath,
		"--ca.key-path", caConfig.KeyPath,
		"--identity.cert-path", identConfig.CertPath,
		"--identity.key-path", identConfig.KeyPath,
	)
	if !assert.NoError(err) {
		t.Fatal(err)
	}
}
