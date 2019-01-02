// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/server"
)

type (
	// Provider Transition: pkg/provider is going away.
	Provider = server.Server
	// ServerConfig Transition: pkg/provider is going away.
	ServerConfig = server.Config
	// FullIdentity Transition: pkg/provider is going away.
	FullIdentity = identity.FullIdentity
	// PeerIdentity Transition: pkg/provider is going away.
	PeerIdentity = identity.PeerIdentity
	// CASetupConfig Transition: pkg/provider is going away.
	CASetupConfig = identity.CASetupConfig
	// PeerCAConfig Transition: pkg/provider is going away.
	PeerCAConfig = identity.PeerCAConfig
	// FullCAConfig Transition: pkg/provider is going away.
	FullCAConfig = identity.FullCAConfig
	// IdentitySetupConfig Transition: pkg/provider is going away.
	IdentitySetupConfig = identity.SetupConfig
	// IdentityConfig Transition: pkg/provider is going away.
	IdentityConfig = identity.Config
	// NewCAOptions Transition: pkg/provider is going away.
	NewCAOptions = identity.NewCAOptions
	// FullCertificateAuthority Transition: pkg/provider is going away.
	FullCertificateAuthority = identity.FullCertificateAuthority
)

var (
	// NewProvider Transition: pkg/provider is going away.
	NewProvider = server.NewServer
	// PeerIdentityFromContext Transition: pkg/provider is going away.
	PeerIdentityFromContext = identity.PeerIdentityFromContext
	// NewFullIdentity Transition: pkg/provider is going away.
	NewFullIdentity = identity.NewFullIdentity
	// SetupIdentity Transition: pkg/provider is going away.
	SetupIdentity = identity.SetupIdentity
	// SetupCA Transition: pkg/provider is going away.
	SetupCA = identity.SetupCA
	// NewCA Transition: pkg/provider is going away.
	NewCA = identity.NewCA
	// ErrSetup Transition: pkg/provider is going away.
	ErrSetup = identity.ErrSetup
	// NoCertNoKey Transition: pkg/provider is going away.
	NoCertNoKey = identity.NoCertNoKey
	// FullIdentityFromPEM Transition: pkg/provider is going away.
	FullIdentityFromPEM = identity.FullIdentityFromPEM
)
