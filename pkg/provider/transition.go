// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/server"
)

type (
	Provider     = server.Server
	ServerConfig = server.ServerConfig

	FullIdentity             = identity.FullIdentity
	PeerIdentity             = identity.PeerIdentity
	CASetupConfig            = identity.CASetupConfig
	PeerCAConfig             = identity.PeerCAConfig
	FullCAConfig             = identity.FullCAConfig
	IdentitySetupConfig      = identity.IdentitySetupConfig
	IdentityConfig           = identity.IdentityConfig
	NewCAOptions             = identity.NewCAOptions
	FullCertificateAuthority = identity.FullCertificateAuthority
)

var (
	NewServerOptions = server.NewServerOptions
	NewProvider      = server.NewServer

	PeerIdentityFromContext = identity.PeerIdentityFromContext
	NewFullIdentity         = identity.NewFullIdentity
	SetupIdentity           = identity.SetupIdentity
	SetupCA                 = identity.SetupCA
	NewCA                   = identity.NewCA
	ErrSetup                = identity.ErrSetup
	NoCertNoKey             = identity.NoCertNoKey
	FullIdentityFromPEM     = identity.FullIdentityFromPEM
)
